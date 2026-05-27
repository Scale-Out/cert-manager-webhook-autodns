package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName,
		&autoDNSProviderSolver{},
	)
}

type autoDNSProviderSolver struct {
	client *kubernetes.Clientset
}

type secretKeyRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type autoDNSProviderConfig struct {
	Zone              string       `json:"zone,omitempty"`
	NameServer        string       `json:"nameserver"`
	Context           string       `json:"context"`
	URL               string       `json:"url"`
	UsernameSecretRef secretKeyRef `json:"usernameSecretRef"`
	PasswordSecretRef secretKeyRef `json:"passwordSecretRef"`
}

type AutoDNSData struct {
	Origin             string                      `json:"origin"`
	ResourceRecord     []AutoDNSResourceRecordData `json:"resourceRecord,omitempty"`
	ResourceRecordsAdd []AutoDNSResourceRecordData `json:"resourceRecordsAdd,omitempty"`
	ResourceRecordsRem []AutoDNSResourceRecordData `json:"resourceRecordsRem,omitempty"`
}

type AutoDNSResourceRecordData struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
	Pref  int64  `json:"pref,omitempty"`
	TTL   int64  `json:"ttl,omitempty"`
}

func (c *autoDNSProviderSolver) Name() string {
	return "autodns"
}

func (c *autoDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}
	if cfg.Zone == "" {
		cfg.Zone = ch.ResolvedZone
	}

	user, pass, err := c.resolveCredentials(ch, cfg)
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(AutoDNSData{
		Origin: ch.ResolvedZone,
		ResourceRecordsAdd: []AutoDNSResourceRecordData{
			{
				Name:  ch.ResolvedFQDN,
				Value: ch.Key,
				TTL:   60,
				Type:  "TXT",
			},
		},
	})
	if err != nil {
		return err
	}

	return callApi("PATCH", jsonData, cfg, user, pass)
}

func (c *autoDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}
	if cfg.Zone == "" {
		cfg.Zone = ch.ResolvedZone
	}

	user, pass, err := c.resolveCredentials(ch, cfg)
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(AutoDNSData{
		Origin: ch.ResolvedZone,
		ResourceRecordsRem: []AutoDNSResourceRecordData{
			{
				Name:  ch.ResolvedFQDN,
				Value: ch.Key,
				TTL:   60,
				Type:  "TXT",
			},
		},
	})
	if err != nil {
		return err
	}

	return callApi("PATCH", jsonData, cfg, user, pass)
}

func (c *autoDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	c.client = cl
	return nil
}

func loadConfig(cfgJSON *extapi.JSON) (autoDNSProviderConfig, error) {
	cfg := autoDNSProviderConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	return cfg, nil
}

func (c *autoDNSProviderSolver) resolveCredentials(ch *v1alpha1.ChallengeRequest, cfg autoDNSProviderConfig) (string, string, error) {
	user, err := c.resolveSecret(ch.ResourceNamespace, cfg.UsernameSecretRef)
	if err != nil {
		return "", "", fmt.Errorf("resolve usernameSecretRef: %w", err)
	}
	pass, err := c.resolveSecret(ch.ResourceNamespace, cfg.PasswordSecretRef)
	if err != nil {
		return "", "", fmt.Errorf("resolve passwordSecretRef: %w", err)
	}
	return user, pass, nil
}

func (c *autoDNSProviderSolver) resolveSecret(namespace string, ref secretKeyRef) (string, error) {
	if ref.Name == "" || ref.Key == "" {
		return "", fmt.Errorf("secretRef name and key must be set")
	}
	sec, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get secret %s/%s: %w", namespace, ref.Name, err)
	}
	v, ok := sec.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", ref.Key, namespace, ref.Name)
	}
	return string(v), nil
}

func callApi(method string, body []byte, cfg autoDNSProviderConfig, user, pass string) error {
	zone := strings.TrimSuffix(cfg.Zone, ".")
	url := cfg.URL + "/zone/" + zone + "/" + cfg.NameServer
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("unable to build request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Domainrobot-Context", cfg.Context)
	req.SetBasicAuth(user, pass)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	bodyStr := string(respBody)

	// EF02022 = duplicate record on add (Present already applied).
	// EF02021 = record not found on remove (CleanUp already applied).
	// cert-manager calls Present/CleanUp repeatedly during reconcile loops;
	// treat these as success to keep both operations idempotent.
	if strings.Contains(bodyStr, "EF02022") || strings.Contains(bodyStr, "EF02021") {
		klog.Infof("AutoDNS idempotent no-op (status=%s body=%s)", resp.Status, bodyStr)
		return nil
	}

	text := fmt.Sprintf("AutoDNS API error: status=%s url=%s method=%s body=%s", resp.Status, url, method, bodyStr)
	klog.Error(text)
	return errors.New(text)
}