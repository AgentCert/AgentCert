package catalog

import (
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
)

// ToGraphQLModel maps an internal AppCatalogEntry to the gqlgen-generated model.ApplicationSpec.
func ToGraphQLModel(e *AppCatalogEntry) *model.ApplicationSpec {
	if e == nil {
		return nil
	}

	spec := &model.ApplicationSpec{
		Name:              e.Metadata.Name,
		DisplayName:       e.Metadata.DisplayName,
		Version:           e.Metadata.Version,
		Tier:              e.Metadata.Tier,
		Domain:            e.Metadata.Domain,
		CapabilityDomains: strSlice(e.Metadata.CapabilityDomains),
		Tags:              strSlice(e.Metadata.Tags),
		SchemaVersion:     "1",
		Description: &model.CatalogAppDescription{
			Short:          e.Spec.Description.Short,
			Long:           e.Spec.Description.Long,
			SuitableFor:    strSlice(e.Spec.Description.SuitableFor),
			NotSuitableFor: strSlice(e.Spec.Description.NotSuitableFor),
		},
		Install: &model.CatalogInstallSpec{
			Method:  e.Spec.Install.Method,
			Folder:  strPtr(e.Spec.Install.Folder),
			Timeout: e.Spec.Install.Timeout,
			Wait:    e.Spec.Install.Wait,
			Namespace: &model.CatalogNamespaceSpec{
				Default:      e.Spec.Install.Namespace.Default,
				Configurable: e.Spec.Install.Namespace.Configurable,
			},
		},
		HealthProbe: &model.CatalogHealthProbeSpec{
			URLTemplate:         e.Spec.HealthProbe.URL,
			ExpectedStatus:      e.Spec.HealthProbe.ExpectedStatus,
			InitialDelaySeconds: defaultInt(e.Spec.HealthProbe.InitialDelaySeconds, 30),
			PeriodSeconds:       defaultInt(e.Spec.HealthProbe.PeriodSeconds, 10),
			FailureThreshold:    defaultInt(e.Spec.HealthProbe.FailureThreshold, 6),
		},
		LoadTest: &model.CatalogLoadTestSpec{
			Enabled: e.Spec.LoadTest.Enabled,
			Method:  strPtr(e.Spec.LoadTest.Method),
			Image:   strPtr(e.Spec.LoadTest.Image),
			Args:    e.Spec.LoadTest.Args,
		},
	}

	if e.Spec.Install.ChartRef != nil {
		spec.Install.ChartRef = &model.CatalogChartRef{
			Repo:    e.Spec.Install.ChartRef.Repo,
			Chart:   e.Spec.Install.ChartRef.Chart,
			Version: e.Spec.Install.ChartRef.Version,
		}
	}

	for _, ms := range e.Spec.Microservices {
		ns := ms.K8s.Namespace
		if ns == "" {
			ns = "{{.AppNamespace}}"
		}
		spec.Microservices = append(spec.Microservices, &model.CatalogMicroserviceSpec{
			Name:           ms.Name,
			DisplayName:    ms.DisplayName,
			Description:    strPtr(ms.Description),
			K8sLabel:       ms.K8s.Label,
			K8sKind:        ms.K8s.Kind,
			K8sNamespace:   ns,
			Criticality:    defaultStr(ms.Criticality, "medium"),
			RelevantFaults: strSlice(ms.RelevantFaults),
			DependsOn:      strSlice(ms.DependsOn),
		})
	}

	for _, fc := range e.Spec.FaultCompatibility {
		spec.FaultCompatibility = append(spec.FaultCompatibility, &model.FaultCompatibilityEntry{
			FaultName:          fc.FaultName,
			Compatible:         fc.Compatible,
			Notes:              strPtr(fc.Notes),
			RecommendedTargets: strSlice(fc.RecommendedTargets),
		})
	}

	for _, inp := range e.Spec.Inputs {
		spec.Inputs = append(spec.Inputs, &model.CatalogAppInput{
			Key:         inp.Key,
			DisplayName: inp.DisplayName,
			Description: strPtr(inp.Description),
			Type:        inp.Type,
			Required:    inp.Required,
			Default:     strPtr(inp.Default),
			HelmPath:    inp.HelmPath,
			Values:      inp.Values,
			Min:         inp.Min,
			Max:         inp.Max,
			Unit:        strPtr(inp.Unit),
			Advanced:    inp.Advanced,
		})
	}

	if spec.Microservices == nil {
		spec.Microservices = []*model.CatalogMicroserviceSpec{}
	}
	if spec.FaultCompatibility == nil {
		spec.FaultCompatibility = []*model.FaultCompatibilityEntry{}
	}
	if spec.Inputs == nil {
		spec.Inputs = []*model.CatalogAppInput{}
	}

	return spec
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
