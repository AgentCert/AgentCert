package chartconfig

import (
	"strings"

	"github.com/litmuschaos/litmus/chaoscenter/graphql/server/graph/model"
	log "github.com/sirupsen/logrus"
)

// LoadModel loads the settings of the chart in chartDir for the GraphQL API.
// A chart whose declarations are invalid is listed with no settings and the
// reason is logged, so one broken chart cannot hide the rest of the catalogue.
func LoadModel(chartDir string) []*model.ConfigField {
	fields, err := Load(chartDir)
	if err != nil {
		log.WithError(err).WithField("chart", chartDir).Error("chart declares invalid configurations; offering no settings")
	}
	return ToModel(fields)
}

// ToModel converts a chart's settings to their GraphQL representation. It
// never returns nil, so a chart without settings is sent as [] rather than null.
func ToModel(fields []Field) []*model.ConfigField {
	out := make([]*model.ConfigField, 0, len(fields))
	for _, field := range fields {
		out = append(out, &model.ConfigField{
			Key:          field.Key,
			Label:        field.Label,
			Description:  optional(field.Description),
			Type:         model.ConfigFieldType(strings.ToUpper(string(field.Type))),
			ValueKind:    model.ConfigValueKind(strings.ToUpper(string(field.Kind))),
			DefaultValue: field.FormatDefault(),
			Required:     field.Required,
			Min:          field.Min,
			Max:          field.Max,
			Pattern:      optional(field.Pattern),
			Options:      field.Options,
			Group:        optional(field.Group),
			Advanced:     field.Advanced,
		})
	}
	return out
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
