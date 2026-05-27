package main

import (
	"os"
	"testing"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
	fqdn = os.Getenv("TEST_FQDN")
)

func TestRunsSuite(t *testing.T) {
	fixture := acmetest.NewFixture(&autoDNSProviderSolver{},
		acmetest.SetResolvedZone(zone),
		acmetest.SetResolvedFQDN(fqdn),
		acmetest.SetAllowAmbientCredentials(false),
		acmetest.SetManifestPath("testdata/autoDNS"),
	)

	fixture.RunConformance(t)
}
