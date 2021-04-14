package powerstore

import (
	"context"
	"os"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/common/tls"
	"github.com/influxdata/telegraf/plugins/inputs"

	"github.com/dell/gopowerstore"
)

// Jenkins plugin gathers information about the nodes and jobs running in a jenkins instance.
type PowerStore struct {
	URL      string
	Username string
	Password string

	tls.ClientConfig

	client gopowerstore.Client

	cancel context.CancelFunc
	Log    telegraf.Logger
}

const sampleConfig = `
  ## The PowerStore REST API URL in the format "schema://host:port"
  url = "https://10.230.24.9/api/rest"
  username = "admin"
  password = "Password123!"

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use SSL but skip chain & host verification
  # insecure_skip_verify = false
`

type ApplianceMetrics struct {
	Timestamp string `json:"timestamp"`
	// Unique identifier of the appliance.
	ApplianceID string `json:"appliance_id"`
	// Total amount of space
	PhysicalTotal int64 `json:"physical_total"`
	// Amount of space currently used
	PhysicalUsed int64 `json:"physical_used"`
}

// measurement
const (
	measurementPowerStoreApplicance = "PowerStoreAppliance"
)

// SampleConfig implements telegraf.Input interface
func (pstore *PowerStore) SampleConfig() string {
	return sampleConfig
}

// Description implements telegraf.Input interface
func (pstore *PowerStore) Description() string {
	return "Collect Dell EMC PowerStore metrics"
}

func (pstore *PowerStore) Start(acc telegraf.Accumulator) error {
	pstore.Log.Info("Starting PowerStore plugin")
	_, cancel := context.WithCancel(context.Background())
	pstore.cancel = cancel

	err := os.Setenv("GOPOWERSTORE_DEBUG", "false")
	if err != nil {
		panic(err)
	}
	clientOptions := gopowerstore.NewClientOptions()
	clientOptions.SetInsecure(true)
	c, err := gopowerstore.NewClientWithArgs(
		pstore.URL,
		pstore.Username,
		pstore.Password,
		clientOptions)
	if err != nil {
		panic(err)
	}

	pstore.client = c

	return nil
}

// Stop is called from telegraf core when a plugin is stopped and allows it to
// perform shutdown tasks.
func (pstore *PowerStore) Stop() {
	pstore.Log.Info("Stopping PowerStore plugin")
	pstore.cancel()
}

// Gather implements telegraf.Input interface
func (pstore *PowerStore) Gather(acc telegraf.Accumulator) error {
	c := pstore.client
	capacity, err := c.GetCapacity(context.Background())
	if err != nil {
		panic(err)
	}
	pstore.Log.Infof("Appliance capacity is %d", capacity)

	var resp []ApplianceMetrics
	client := c.APIClient()
	qp := client.QueryParams().Select("physical_total", "physical_used", "timestamp")
	_, err = client.Query(
		context.Background(),
		gopowerstore.RequestConfig{
			Method:      "POST",
			Endpoint:    "metrics",
			Action:      "generate",
			QueryParams: qp,
			Body: &gopowerstore.MetricsRequest{
				Entity:   "space_metrics_by_appliance",
				EntityID: "A1",
				Interval: "Five_Mins",
			},
		},
		&resp)

	if err != nil {
		panic(err)
	}

	tags := map[string]string{}
	fields := make(map[string]interface{})

	if len(resp) > 0 {
		pstore.Log.Infof("Found %d records for space_metrics_by_appliance", len(resp))
		for _, ametric := range resp {

			fields["physical_total"] = ametric.PhysicalTotal
			fields["physical_used"] = ametric.PhysicalUsed
			tags["Appliance_ID"] = ametric.ApplianceID
			timestamp, err := time.Parse(time.RFC3339, ametric.Timestamp)
			if err == nil {
				acc.AddFields(measurementPowerStoreApplicance, fields, tags, timestamp)
			} else {
				continue
			}

		}
	}

	return nil
}

func init() {
	inputs.Add("powerstore", func() telegraf.Input {
		return &PowerStore{}
	})
}
