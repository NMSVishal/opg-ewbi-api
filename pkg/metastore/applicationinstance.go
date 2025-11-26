package metastore

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neonephos-katalis/opg-ewbi-api/api/federation/models"
	camara "github.com/neonephos-katalis/opg-ewbi-api/api/federation/server"
	opgv1beta1 "github.com/neonephos-katalis/opg-ewbi-operator/api/v1beta1"
	"github.com/labstack/gommon/log"
)

type ApplicationInstanceDetails struct {
	*camara.GetAppInstanceDetails200JSONResponse
}

type ApplicationInstance struct {
	*models.InstallAppJSONBody
	FederationContextId models.FederationContextId `json:"-"`
}

// create fun for GetAppInstanceDetails200JSONResponse
func newApplicationInstanceDetailsFromK8sCR(obj *opgv1beta1.ApplicationInstance) *ApplicationInstanceDetails {
	// fetch exisiting application instance k8s object using  k8sobject getKubernetesObject and return ApplicationInstanceDetails
	appinst := &ApplicationInstanceDetails{
		GetAppInstanceDetails200JSONResponse: &camara.GetAppInstanceDetails200JSONResponse{
			AppInstanceState: (*models.InstanceState)(&obj.Status.State),
			AccesspointInfo:  convertAccessPoint(obj.Status.AccessPointInfo),
		},
	}

	log.Info("Converted ApplicationInstanceDetails from K8s CR successfully")
	log.Info(fmt.Sprintf("%+v", appinst))
	log.Info("AccessPointInfo:")
	log.Info(fmt.Sprintf("%+v", appinst.AccesspointInfo))
	log.Info("AppInstanceState:")
	log.Info(fmt.Sprintf("%+v", appinst.AppInstanceState))

	return appinst
}

func (d *ApplicationInstance) k8sCustomResource(namespace string, opts ...Opt) (*opgv1beta1.ApplicationInstance, error) {
	obj := &opgv1beta1.ApplicationInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sCustomResourceNameFromApplicationInstance(d.FederationContextId, d.AppInstanceId),
			Namespace: namespace,
			Labels: map[string]string{
				opgLabel(federationContextIDLabel): d.FederationContextId,
				opgLabel(idLabel):                  d.AppInstanceId,
				opgLabel(federationRelation):       host,
			},
		},
		Spec: opgv1beta1.ApplicationInstanceSpec{
			AppProviderId: d.AppProviderId,
			AppId:         d.AppId,
			AppVersion:    d.AppVersion,
			ZoneInfo: opgv1beta1.Zone{
				ZoneId:              d.ZoneInfo.ZoneId,
				FlavourId:           d.ZoneInfo.FlavourId,
				ResourceConsumption: defaultIfNil((*string)(d.ZoneInfo.ResourceConsumption)),
				ResPool:             defaultIfNil(d.ZoneInfo.ResPool),
			},
			CallbBackLink: d.AppInstCallbackLink,
		},
	}
	for _, opt := range opts {
		if err := opt(&obj.ObjectMeta); err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func isValidApplicationInstanceStatus(status string) bool {
	switch opgv1beta1.ApplicationInstanceState(status) {
	case opgv1beta1.ApplicationInstanceStatePending, opgv1beta1.ApplicationInstanceStateReady, opgv1beta1.ApplicationInstanceStateFailed, opgv1beta1.ApplicationInstanceStateTerminating:
		return true
	}
	return false
}

func k8sCustomResourceNameFromApplicationInstance(federationContextID, appID string) string {
	return fmt.Sprintf("%s-%s", applicationInstancePrefix, uuidV5Fn(federationContextID+"/"+appID))
}

func convertAccessPoint(opgAccessPoint opgv1beta1.AccessPointInfo) *models.AccessPointInfo {
	// Map opgv1beta1.AccessPointInfo to models.AccessPointInfo
	log.Info("Converting AccessPointInfo")
	return &models.AccessPointInfo{
		{
			AccessPoints: models.ServiceEndpoint{
				Port:          opgAccessPoint.AccessPoint.Port,
				Fqdn:          &opgAccessPoint.AccessPoint.Fqdn,
				Ipv4Addresses: &[]models.Ipv4Addr{models.Ipv4Addr(opgAccessPoint.AccessPoint.Ipv4Addresses)},
				Ipv6Addresses: &[]models.Ipv6Addr{models.Ipv6Addr(opgAccessPoint.AccessPoint.Ipv6Addresses)},
			},
			InterfaceId: models.InterfaceId(opgAccessPoint.InterfaceId),
		},
	}
}
