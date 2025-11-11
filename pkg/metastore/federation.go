package metastore

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neonephos-katalis/opg-ewbi-api/api/federation/models"
	opgv1beta1 "github.com/neonephos-katalis/opg-ewbi-operator/api/v1beta1"
)

type Federation struct {
	*models.FederationRequestData
	ClientCredentials         ClientCredentials
	FederationContextId       models.FederationContextId
	AcceptedAvailabilityZones *[]models.ZoneIdentifier
	OfferedAvailabilityZones  *[]models.ZoneDetails
}

func (f *Federation) updatek8sCustomResource(fed *opgv1beta1.Federation) *opgv1beta1.Federation {
	var aaz []string

	// we can interact with TIM inventory here and update the offered availability zones
	if f.AcceptedAvailabilityZones != nil {
		aaz = *f.AcceptedAvailabilityZones
	}

	fed.ObjectMeta.Labels[opgLabel(federationContextIDLabel)] = f.FederationContextId
	fed.ObjectMeta.Labels[opgLabel(idLabel)] = f.FederationContextId
	fed.ObjectMeta.Labels[opgLabel(federationRelation)] = host
	fed.Spec.InitialDate = metav1.Time{Time: f.InitialDate}
	fed.Spec.OriginOP = opgv1beta1.Origin{
		CountryCode:       defaultIfNil(f.OrigOPCountryCode),
		FixedNetworkCodes: *f.OrigOPFixedNetworkCodes,
		MobileNetworkCodes: opgv1beta1.MobileNetworkCodes{
			MCC: *f.OrigOPMobileNetworkCodes.Mcc,
			MNC: *f.OrigOPMobileNetworkCodes.Mncs,
		},
	}
	fed.Spec.Partner = opgv1beta1.Partner{
		CallbackCredentials: opgv1beta1.FederationCredentials{
			ClientId: f.PartnerCallbackCredentials.ClientId,
			TokenUrl: f.PartnerCallbackCredentials.TokenUrl,
		},
		StatusLink: f.PartnerStatusLink,
	}
	fed.Spec.AcceptedAvailabilityZones = aaz
	return fed
}

func federationFromK8sCustomResource(fed *opgv1beta1.Federation) (*Federation, error) {
	offeredZones := make([]models.ZoneDetails, len(fed.Spec.OfferedAvailabilityZones))
	for i, z := range fed.Spec.OfferedAvailabilityZones {
		offeredZones[i].ZoneId = z
		// We can enhance this by fetching more details from TIM inventory like zone geographyDetails ,zone latitiude, longitude etc.
	}

	// iterate offeredZones ,for each offerZone  call inventory GET API ,pass zoneId , inventory GET API response will have json as {
	//     "geographyDetails": "aws,Milan",
	//     "geolocation": "45.4642,9.19"
	// }
	// parse geographyDetails ,geolocation  and populate offeredZones[i].GeographyDetails and offeredZones[i].GeoLocation fields.

	for i := 0; i < len(offeredZones); i++ {

		zoneid := offeredZones[i].ZoneId
		log.Printf(" ######### Fetching Geolocation and GGeographyDetails from TIM Inventory For ZoneID: %s", zoneid)

		inventoryClient := GetInventoryAPIClient()

		resp, err := inventoryClient.GetZoneDetails(zoneid)
		if err != nil {
			return nil, err
		}
		offeredZones[i].GeographyDetails = resp.GeographyDetails
		offeredZones[i].Geolocation = resp.Geolocation
		log.Printf(" ######### Successfully Completed Interaction with TIM Inventory For ZoneID: %s", zoneid)

	}

	return &Federation{
		FederationRequestData: &models.FederationRequestData{
			InitialDate:             fed.Spec.InitialDate.Time,
			OrigOPCountryCode:       &fed.Spec.OriginOP.CountryCode,
			OrigOPFixedNetworkCodes: &fed.Spec.OriginOP.FixedNetworkCodes,
			OrigOPMobileNetworkCodes: &models.MobileNetworkIds{
				Mcc:  &fed.Spec.OriginOP.MobileNetworkCodes.MCC,
				Mncs: &fed.Spec.OriginOP.MobileNetworkCodes.MNC,
			},
			PartnerCallbackCredentials: &models.CallbackCredentials{
				ClientId: fed.Spec.Partner.CallbackCredentials.ClientId,
				TokenUrl: fed.Spec.Partner.CallbackCredentials.TokenUrl,
			},
		},
		FederationContextId:       fed.Labels[opgLabel(federationContextIDLabel)],
		OfferedAvailabilityZones:  &offeredZones,
		AcceptedAvailabilityZones: &fed.Spec.AcceptedAvailabilityZones,
	}, nil
}

func isValidFederationStatus(status string) bool {
	switch opgv1beta1.FederationState(status) {
	case opgv1beta1.FederationStateFailed, opgv1beta1.FederationStateTemporaryFailure, opgv1beta1.FederationStateAvailable, opgv1beta1.FederationStateLocked, opgv1beta1.FederationStateNotAvailable:
		return true
	}
	return false
}

func GetInventoryAPIClient() *InventoryAPIClient {
	baseURL := os.Getenv("INVENTORY_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://10.10.0.85:5000/inventory/api/v1" // Dult value if env variable is not set
	}

	return &InventoryAPIClient{
		BaseURL:    baseURL,
		HTTPClient: &HTTPClient{},
	}
}

type InventoryAPIClient struct {
	BaseURL    string
	HTTPClient *HTTPClient
}

type HTTPClient struct{}

func (c *HTTPClient) Get(url string, response interface{}) error {
	log.Printf("Initiating GET request to URL: %s", url)

	// Create a new HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Error creating HTTP request: %v", err)
		return err
	}

	// Execute the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error executing HTTP request: %v", err)
		return err
	}
	defer resp.Body.Close()

	log.Printf("Received HTTP response with status: %s", resp.Status)

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("HTTP request failed with status: %s", resp.Status)
		return fmt.Errorf("HTTP request failed with status: %s", resp.Status)
	}

	// Decode the response body into the provided response object
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(response); err != nil {
		log.Printf("Error decoding HTTP response: %v", err)
		return err
	}

	log.Printf("Successfully decoded HTTP response for URL: %s", url)
	return nil
}

type ZoneDetailsResponse struct {
	GeographyDetails string `json:"geographyDetails"`
	Geolocation      string `json:"geolocation"`
}

func (client *InventoryAPIClient) GetZoneDetails(zoneID string) (*ZoneDetailsResponse, error) {
	url := fmt.Sprintf("%s/zone-details?zoneid=%s", client.BaseURL, zoneID)
	log.Printf(" ######### Initiating GET request to TIM Inventory URL: %s", url)
	var zoneDetails ZoneDetailsResponse
	if err := client.HTTPClient.Get(url, &zoneDetails); err != nil {
		return nil, err
	}
	return &zoneDetails, nil
}
