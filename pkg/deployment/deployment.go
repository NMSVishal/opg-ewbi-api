package deployment

import (
	"github.com/NMSVishal/opg-ewbi-api/api/federation/models"
)

type InstallDeployment struct {
	*models.InstallAppJSONBody
	FederationContextID string
}
