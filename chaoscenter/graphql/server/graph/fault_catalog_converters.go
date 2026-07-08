package graph

import (
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/pkg/fault_catalog"
)

func faultEntryToGraphQL(e *fault_catalog.FaultCatalogEntry) *model.FaultSpec {
	if e == nil {
		return nil
	}

	params := make([]*model.FaultParameter, len(e.Spec.Parameters))
	for i, p := range e.Spec.Parameters {
		pp := &model.FaultParameter{
			Key:         p.Key,
			DisplayName: p.DisplayName,
			Type:        faultParamTypeToGraphQL(p.Type),
			Default:     p.Default,
			Required:    p.Required,
			Description: p.Description,
		}
		if p.Unit != "" {
			pp.Unit = &p.Unit
		}
		if p.Min != nil {
			pp.Min = p.Min
		}
		if p.Max != nil {
			pp.Max = p.Max
		}
		if p.LitmusEnv != "" {
			pp.LitmusEnv = &p.LitmusEnv
		}
		if len(p.AllowedValues) > 0 {
			pp.AllowedValues = p.AllowedValues
		}
		params[i] = pp
	}

	implType := faultImplTypeToGraphQL(e.Spec.Implementation.Type)
	impl := &model.FaultImplementation{
		Type: implType,
	}
	if e.Spec.Implementation.ChaosKind != "" {
		impl.ChaosKind = &e.Spec.Implementation.ChaosKind
	}
	if e.Spec.Implementation.ExperimentRef != "" {
		impl.ExperimentRef = &e.Spec.Implementation.ExperimentRef
	}
	if e.Spec.Implementation.Namespace != "" {
		impl.Namespace = &e.Spec.Implementation.Namespace
	}
	if e.Spec.Implementation.Image != "" {
		impl.Image = &e.Spec.Implementation.Image
	}
	if e.Spec.Implementation.Endpoint != "" {
		impl.Endpoint = &e.Spec.Implementation.Endpoint
	}

	gtCat := faultGTCatToGraphQL(e.Spec.GroundTruth.Category)

	spec := &model.FaultSpec{
		Name:        e.Metadata.Name,
		DisplayName: e.Metadata.DisplayName,
		Version:     e.Metadata.Version,
		Tier:        faultTierToGraphQL(e.Metadata.Tier),
		Scope:       faultScopeToGraphQL(e.Metadata.Scope),
		Tags:        e.Metadata.Tags,
		FilePath:    e.FilePath,
		Description: &model.FaultDescription{
			Short:          e.Spec.Description.Short,
			Long:           e.Spec.Description.Long,
			SuitableFor:    e.Spec.Description.SuitableFor,
			NotSuitableFor: e.Spec.Description.NotSuitableFor,
		},
		Implementation: impl,
		Parameters:     params,
		Compatibility: &model.FaultCompatibility{
			TargetDomains:        e.Spec.Compatibility.TargetDomains,
			IncompatibleApps:     e.Spec.Compatibility.IncompatibleApps,
			RequiredCapabilities: e.Spec.Compatibility.RequiredCapabilities,
		},
		Observability: &model.FaultObservability{
			ExpectedSymptoms:    e.Spec.Observability.ExpectedSymptoms,
			ExpectedAlerts:      e.Spec.Observability.ExpectedAlerts,
			DetectionWindowSecs: e.Spec.Observability.DetectionWindowSecs,
		},
		GroundTruth: &model.FaultGroundTruth{
			Category:           gtCat,
			Impact:             e.Spec.GroundTruth.Impact,
			DetectWithinSecs:   e.Spec.GroundTruth.DetectWithinSecs,
			MitigateWithinSecs: e.Spec.GroundTruth.MitigateWithinSecs,
			DetectionHints:     e.Spec.GroundTruth.DetectionHints,
			RemediationHints:   e.Spec.GroundTruth.RemediationHints,
		},
	}
	if e.Metadata.Domain != nil {
		spec.Domain = e.Metadata.Domain
	}
	if e.Metadata.TargetApp != nil {
		spec.TargetApp = e.Metadata.TargetApp
	}
	return spec
}

func faultEntriesToGraphQL(entries []*fault_catalog.FaultCatalogEntry) []*model.FaultSpec {
	out := make([]*model.FaultSpec, 0, len(entries))
	for _, e := range entries {
		if s := faultEntryToGraphQL(e); s != nil {
			out = append(out, s)
		}
	}
	return out
}

func graphqlScopeToService(scope model.FaultScope) fault_catalog.FaultScope {
	switch scope {
	case model.FaultScopeGeneral:
		return fault_catalog.ScopeGeneral
	case model.FaultScopeDomain:
		return fault_catalog.ScopeDomain
	case model.FaultScopeAppSpecific:
		return fault_catalog.ScopeAppSpecific
	default:
		return ""
	}
}

func faultScopeToGraphQL(scope fault_catalog.FaultScope) model.FaultScope {
	switch scope {
	case fault_catalog.ScopeGeneral:
		return model.FaultScopeGeneral
	case fault_catalog.ScopeDomain:
		return model.FaultScopeDomain
	case fault_catalog.ScopeAppSpecific:
		return model.FaultScopeAppSpecific
	default:
		return model.FaultScopeGeneral
	}
}

func faultTierToGraphQL(tier fault_catalog.CatalogTier) model.CatalogTier {
	switch tier {
	case fault_catalog.TierCommunity:
		return model.CatalogTierCommunity
	default:
		return model.CatalogTierOfficial
	}
}

func faultImplTypeToGraphQL(t fault_catalog.FaultImplementationType) model.FaultImplementationType {
	switch t {
	case fault_catalog.ImplHTTPFault:
		return model.FaultImplementationTypeHTTPFault
	case fault_catalog.ImplScript:
		return model.FaultImplementationTypeScript
	case fault_catalog.ImplExternal:
		return model.FaultImplementationTypeExternal
	default:
		return model.FaultImplementationTypeLitmus
	}
}

func faultParamTypeToGraphQL(t fault_catalog.ParameterType) model.FaultParameterType {
	switch t {
	case fault_catalog.ParamTypeBoolean:
		return model.FaultParameterTypeBoolean
	case fault_catalog.ParamTypeEnum:
		return model.FaultParameterTypeEnum
	case fault_catalog.ParamTypePercent:
		return model.FaultParameterTypePercent
	case fault_catalog.ParamTypeString:
		return model.FaultParameterTypeString
	default:
		return model.FaultParameterTypeInteger
	}
}

func faultGTCatToGraphQL(cat fault_catalog.GroundTruthCategory) model.GroundTruthCategory {
	switch cat {
	case fault_catalog.GTCatPerformance:
		return model.GroundTruthCategoryPerformance
	case fault_catalog.GTCatSecurity:
		return model.GroundTruthCategorySecurity
	case fault_catalog.GTCatDataIntegrity:
		return model.GroundTruthCategoryDataIntegrity
	case fault_catalog.GTCatConfiguration:
		return model.GroundTruthCategoryConfiguration
	default:
		return model.GroundTruthCategoryAvailability
	}
}
