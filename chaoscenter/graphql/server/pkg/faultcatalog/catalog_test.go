package faultcatalog

import "testing"

func testApps() []Application {
	return []Application{
		{Key: "sock-shop", Folders: []string{"sock-shop"}, Namespace: "sock-shop", LabelKey: "name"},
		{Key: "bookinfo", Folders: []string{"bookinfo", "book-info"}, Namespace: "book-info", LabelKey: "app"},
		{Key: "otel-demo", Folders: []string{"otel-demo"}, Namespace: "otel-demo", LabelKey: "opentelemetry.io/name"},
	}
}

func testFaults() map[string]rawFault {
	return map[string]rawFault{
		"pod-delete": {
			Classification:     ClassificationGeneric,
			TargetRequirements: rawTargetRequirements{WorkloadKinds: []string{"deployment"}},
		},
		"opentelemetry-demo-feature-flag": {
			Classification: ClassificationAppSpecific,
			TargetRequirements: rawTargetRequirements{
				RequiredApp:      "otel-demo",
				RequiredServices: []string{"flagd"},
			},
		},
		"pins-a-retired-app": {
			Classification:     ClassificationAppSpecific,
			TargetRequirements: rawTargetRequirements{RequiredApp: "no-such-app"},
		},
	}
}

// Onboarding an application must widen every generic fault with no other edit.
func TestGenericFaultCoversEveryRegisteredApplication(t *testing.T) {
	c := Build(testApps(), testFaults())

	for _, app := range []string{"sock-shop", "bookinfo", "otel-demo"} {
		if !c.IsFaultCompatible("pod-delete", app) {
			t.Errorf("generic fault should be compatible with %q", app)
		}
	}

	withNewApp := append(testApps(), Application{
		Key: "shop-demo", Folders: []string{"shop-demo"}, Namespace: "shop-demo", LabelKey: "app",
	})
	if !Build(withNewApp, testFaults()).IsFaultCompatible("pod-delete", "shop-demo") {
		t.Error("a newly onboarded application must be compatible with generic faults automatically")
	}
}

func TestApplicationSpecificFaultIsPinned(t *testing.T) {
	c := Build(testApps(), testFaults())

	if !c.IsFaultCompatible("opentelemetry-demo-feature-flag", "otel-demo") {
		t.Error("pinned fault must be compatible with the app it pins")
	}
	if c.IsFaultCompatible("opentelemetry-demo-feature-flag", "sock-shop") {
		t.Error("pinned fault must not be compatible with another app")
	}
}

// Onboarding a fault must not require a catalog entry before it can be used.
func TestUncataloguedFaultIsUnrestricted(t *testing.T) {
	c := Build(testApps(), testFaults())
	if !c.IsFaultCompatible("brand-new-fault", "sock-shop") {
		t.Error("a fault absent from the catalog must be treated as generic")
	}
}

// A pin to a removed application must fail loudly (empty set), not silently
// widen to every application.
func TestPinToUnknownApplicationDerivesEmpty(t *testing.T) {
	c := Build(testApps(), testFaults())
	fault, ok := c.Lookup("pins-a-retired-app")
	if !ok {
		t.Fatal("fault should be present in the catalog")
	}
	if len(fault.CompatibleApps) != 0 {
		t.Errorf("want empty compatible-app set, got %v", fault.CompatibleApps)
	}
}

func TestResolveApplicationPrefersFolderOverNamespace(t *testing.T) {
	c := Build(testApps(), testFaults())

	// A renamed chart's alias must still resolve.
	if app, ok := c.ResolveApplication("book-info", ""); !ok || app.Key != "bookinfo" {
		t.Errorf("alias lookup failed: got %+v ok=%v", app, ok)
	}
	// The operator overrode the namespace; the folder still decides.
	if app, ok := c.ResolveApplication("sock-shop", "team-a-sock-shop"); !ok || app.Key != "sock-shop" {
		t.Errorf("folder should win over namespace: got %+v ok=%v", app, ok)
	}
	// Namespace-only resolution is the fallback for hand-edited manifests.
	if app, ok := c.ResolveApplication("", "otel-demo"); !ok || app.Key != "otel-demo" {
		t.Errorf("namespace fallback failed: got %+v ok=%v", app, ok)
	}
	if _, ok := c.ResolveApplication("unknown", "unknown"); ok {
		t.Error("unknown folder and namespace must not resolve")
	}
}

func TestAgentCompatibilityDefaultsToUnrestricted(t *testing.T) {
	if !IsAgentCompatible(nil, "any-app-onboarded-later") {
		t.Error("an agent declaring nothing must work with every application")
	}
	if IsAgentCompatible([]string{}, "sock-shop") {
		t.Error("an explicitly empty list means not application-targeted")
	}
	if !IsAgentCompatible([]string{"sock-shop"}, "sock-shop") {
		t.Error("declared application must be compatible")
	}
	if IsAgentCompatible([]string{"sock-shop"}, "otel-demo") {
		t.Error("undeclared application must not be compatible")
	}
}
