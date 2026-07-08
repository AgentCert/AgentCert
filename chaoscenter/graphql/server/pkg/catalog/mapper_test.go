package catalog

import (
	"testing"
)

func TestToGraphQLModel_Nil(t *testing.T) {
	if got := ToGraphQLModel(nil); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

func TestToGraphQLModel_ScalarFields(t *testing.T) {
	entry := &AppCatalogEntry{
		Metadata: AppMetadata{
			Name:              "sock-shop",
			DisplayName:       "Sock Shop",
			Version:           "1.0.0",
			Tier:              "official",
			Domain:            "e-commerce",
			CapabilityDomains: []string{"resilience"},
			Tags:              []string{"demo"},
		},
		Spec: AppSpec{
			Description: AppDescription{
				Short:          "Short desc",
				Long:           "Long desc",
				SuitableFor:    []string{"demos"},
				NotSuitableFor: []string{"prod"},
			},
			Install: InstallSpec{
				Method:  "helm",
				Folder:  "sock-shop",
				Timeout: "5m",
				Wait:    true,
				Namespace: NamespaceSpec{
					Default:      "sock-shop",
					Configurable: true,
				},
			},
			HealthProbe: HealthProbeSpec{
				URL:                 "http://front-end.{{.AppNamespace}}.svc:80",
				ExpectedStatus:      "200",
				InitialDelaySeconds: 30,
				PeriodSeconds:       10,
				FailureThreshold:    6,
			},
			LoadTest: LoadTestSpec{
				Enabled: false,
			},
		},
	}

	m := ToGraphQLModel(entry)

	if m.Name != "sock-shop" {
		t.Errorf("Name: want sock-shop, got %s", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version: want 1.0.0, got %s", m.Version)
	}
	if m.SchemaVersion != "1" {
		t.Errorf("SchemaVersion: want 1, got %s", m.SchemaVersion)
	}
	if m.Description == nil || m.Description.Short != "Short desc" {
		t.Errorf("Description.Short mismatch")
	}
	if m.Install == nil || m.Install.Method != "helm" {
		t.Errorf("Install.Method mismatch")
	}
	if m.Install.Namespace == nil || m.Install.Namespace.Default != "sock-shop" {
		t.Errorf("Install.Namespace.Default mismatch")
	}
	if m.HealthProbe == nil || m.HealthProbe.URLTemplate != "http://front-end.{{.AppNamespace}}.svc:80" {
		t.Errorf("HealthProbe.URLTemplate mismatch")
	}
}

func TestToGraphQLModel_DefaultIntsApplied(t *testing.T) {
	entry := &AppCatalogEntry{
		Spec: AppSpec{
			HealthProbe: HealthProbeSpec{
				// All zero — defaults should kick in
			},
		},
	}
	m := ToGraphQLModel(entry)
	if m.HealthProbe.InitialDelaySeconds != 30 {
		t.Errorf("InitialDelaySeconds default: want 30, got %d", m.HealthProbe.InitialDelaySeconds)
	}
	if m.HealthProbe.PeriodSeconds != 10 {
		t.Errorf("PeriodSeconds default: want 10, got %d", m.HealthProbe.PeriodSeconds)
	}
	if m.HealthProbe.FailureThreshold != 6 {
		t.Errorf("FailureThreshold default: want 6, got %d", m.HealthProbe.FailureThreshold)
	}
}

func TestToGraphQLModel_EmptySlicesNotNil(t *testing.T) {
	entry := &AppCatalogEntry{}
	m := ToGraphQLModel(entry)

	if m.CapabilityDomains == nil {
		t.Error("CapabilityDomains should not be nil")
	}
	if m.Tags == nil {
		t.Error("Tags should not be nil")
	}
	if m.Microservices == nil {
		t.Error("Microservices should not be nil")
	}
	if m.FaultCompatibility == nil {
		t.Error("FaultCompatibility should not be nil")
	}
	if m.Inputs == nil {
		t.Error("Inputs should not be nil")
	}
}

func TestToGraphQLModel_ChartRef(t *testing.T) {
	entry := &AppCatalogEntry{
		Spec: AppSpec{
			Install: InstallSpec{
				ChartRef: &ChartRef{
					Repo:    "https://example.com/charts",
					Chart:   "sock-shop",
					Version: "1.2.3",
				},
			},
		},
	}
	m := ToGraphQLModel(entry)
	if m.Install.ChartRef == nil {
		t.Fatal("ChartRef should not be nil")
	}
	if m.Install.ChartRef.Version != "1.2.3" {
		t.Errorf("ChartRef.Version: want 1.2.3, got %s", m.Install.ChartRef.Version)
	}
}

func TestToGraphQLModel_MicroserviceDefaultNamespace(t *testing.T) {
	entry := &AppCatalogEntry{
		Spec: AppSpec{
			Microservices: []MicroserviceSpec{
				{
					Name:        "carts",
					DisplayName: "Carts",
					K8s:         K8sSpec{Label: "name=carts", Kind: "Deployment", Namespace: ""},
				},
			},
		},
	}
	m := ToGraphQLModel(entry)
	if len(m.Microservices) != 1 {
		t.Fatalf("want 1 microservice, got %d", len(m.Microservices))
	}
	if m.Microservices[0].K8sNamespace != "{{.AppNamespace}}" {
		t.Errorf("empty namespace should default to {{.AppNamespace}}, got %s", m.Microservices[0].K8sNamespace)
	}
}

func TestToGraphQLModel_MicroserviceCriticalityDefault(t *testing.T) {
	entry := &AppCatalogEntry{
		Spec: AppSpec{
			Microservices: []MicroserviceSpec{
				{Name: "carts", K8s: K8sSpec{Namespace: "ns"}},
			},
		},
	}
	m := ToGraphQLModel(entry)
	if m.Microservices[0].Criticality != "medium" {
		t.Errorf("empty criticality should default to medium, got %s", m.Microservices[0].Criticality)
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Error("empty string should return nil pointer")
	}
	p := strPtr("hello")
	if p == nil || *p != "hello" {
		t.Error("non-empty string should return pointer to value")
	}
}
