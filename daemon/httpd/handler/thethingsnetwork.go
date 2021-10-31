package handler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	DownlinkPriorityNormal = "NORMAL"
	// AppCommandPort is the magic LoRaWAN port number for a transmitter to
	// transmit an app command.
	AppCommandPort = 112
)

type ApplicationIDs struct {
	ApplicationID string `json:"application_id"`
}

type EndDeviceIDs struct {
	ApplicationIDs ApplicationIDs `json:"application_ids"`
	DeviceID       string         `json:"device_id"`
	DeviceEUI      string         `json:"dev_eui"`
	DeviceAddr     string         `json:"dev_addr"`
}

type PacketBroker struct {
	MessageID          string `json:"message_id"`
	ForwarderGatewayID string `json:"forwarder_gateway_id"`
}

type LORADataRate struct {
	Bandwidth       int `json:"bandwidth"`
	SpreadingFactor int `json:"spreading_factor"`
}

type DataRateSettings struct {
	LORA LORADataRate `json:"lora"`
}

type MessageSettings struct {
	DataRate      DataRateSettings `json:"data_rate"`
	DataRateIndex int              `json:"data_rate_index"`
	Frequency     string           `json:"frequency"`
}

type GatewayMetadata struct {
	ReceptionTime string       `json:"time"`
	RSSI          float64      `json:"rssi"`
	SNR           float64      `json:"snr"`
	PacketBroker  PacketBroker `json:"packet_broker"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
	Accuracy  float64 `json:"accuracy"`
	Source    string  `json:"source"`
}

type Locations struct {
	LocationFromPayload Location `json:"frm-payload"`
}

type UplinkMessage struct {
	PortNumber       int               `json:"f_port"`
	Counter          int               `json:"f_cnt"`
	RawPayloadBase64 string            `json:"frm_payload"`
	GatewayMetadata  []GatewayMetadata `json:"rx_metadata"`
	MesageSettings   MessageSettings   `json:"settings"`
	Locations        Locations         `json:"locations"`
	ConsumedAirtime  string            `json:"consumed_airtime"`
}

type WebHookPayload struct {
	EndDeviceIDs        EndDeviceIDs  `json:"end_device_ids"`
	ReceivedByGatewayAt string        `json:"received_at"`
	UplinkMessage       UplinkMessage `json:"uplink_message"`
}

// ReceptionComment describes a reception of TTN message by a gateway.
// The comment will be stored by the message processor in-memory.
type ReceptionComment struct {
	DeviceID                      string
	DeviceAddr                    string
	UplinkCounter                 int
	UplinkPort                    int
	Latitude, Longitude, Altitude float64
	Frequency                     string
	SpreadingFactor               int
	Bandwidth                     int
	NumGateways                   int
	GatewayID                     string
	RSSI                          float64
	SNR                           float64
	TimeAtReception               string
}

/*
HandleTheThingsNetworkHTTPIntegration collects an uplink message from TheThingsNetwork HTTP integration endpoint,
if the message carries an app command, the command will be executed by store&forward command processor, and the result
will be delivered as a downlink message.
*/
type HandleTheThingsNetworkHTTPIntegration struct {
	cmdProc *toolbox.CommandProcessor
	logger  lalog.Logger
}

func (hand *HandleTheThingsNetworkHTTPIntegration) Initialise(logger lalog.Logger, cmdProc *toolbox.CommandProcessor, _ string) error {
	if cmdProc == nil {
		return errors.New("HandleTheThingsNetworkHTTPIntegration.Initialise: command processor must not be nil")
	}
	if errs := cmdProc.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("HandleTheThingsNetworkHTTPIntegration.Initialise: %+v", errs)
	}
	hand.cmdProc = cmdProc
	hand.logger = logger
	return nil
}

type DownlinkMessage struct {
	Port             int    `json:"f_port"`
	RawPayloadBase64 string `json:"frm_payload"`
	Priority         string `json:"priority"`
}

type Downlinks struct {
	DownlinkMessage []DownlinkMessage `json:"downlinks"`
}

func (msg Downlinks) JSONString() string {
	b, err := json.Marshal(msg)
	if err != nil {
		lalog.DefaultLogger.Warning("Downlinks.JSONString", "", err, "failed to marshal message")
		return ""
	}
	return string(b)
}

func (hand *HandleTheThingsNetworkHTTPIntegration) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()
	var uplinkInfo WebHookPayload
	if err := json.Unmarshal(body, &uplinkInfo); err != nil || len(uplinkInfo.EndDeviceIDs.DeviceID) == 0 {
		http.Error(w, "failed to decode uplink message", http.StatusBadRequest)
		return
	}
	// Decode the raw payload sent by transmitter
	payloadBytes, err := base64.StdEncoding.DecodeString(uplinkInfo.UplinkMessage.RawPayloadBase64)
	if err != nil {
		http.Error(w, "failed to decode uplink message payload", http.StatusBadRequest)
		return
	}
	// Construct a report to save to message processor
	var firstGW GatewayMetadata
	if len(uplinkInfo.UplinkMessage.GatewayMetadata) > 0 {
		firstGW = uplinkInfo.UplinkMessage.GatewayMetadata[0]
	}
	hand.logger.Info("Handle", uplinkInfo.EndDeviceIDs.DeviceID, nil,
		"received transmission from device EUI %s (addr %s) packet #%d on port %d, located at %f, %f, received by gateway %s (RSSI %f), payload size %d bytes.",
		uplinkInfo.EndDeviceIDs.DeviceEUI, uplinkInfo.EndDeviceIDs.DeviceAddr,
		uplinkInfo.UplinkMessage.Counter, uplinkInfo.UplinkMessage.PortNumber,
		uplinkInfo.UplinkMessage.Locations.LocationFromPayload.Latitude, uplinkInfo.UplinkMessage.Locations.LocationFromPayload.Longitude,
		firstGW.PacketBroker.ForwarderGatewayID, firstGW.RSSI,
		len(payloadBytes))

	comment := ReceptionComment{
		DeviceID:        uplinkInfo.EndDeviceIDs.DeviceID,
		DeviceAddr:      uplinkInfo.EndDeviceIDs.DeviceAddr,
		UplinkCounter:   uplinkInfo.UplinkMessage.Counter,
		UplinkPort:      uplinkInfo.UplinkMessage.PortNumber,
		Latitude:        uplinkInfo.UplinkMessage.Locations.LocationFromPayload.Latitude,
		Longitude:       uplinkInfo.UplinkMessage.Locations.LocationFromPayload.Longitude,
		Altitude:        uplinkInfo.UplinkMessage.Locations.LocationFromPayload.Altitude,
		Frequency:       uplinkInfo.UplinkMessage.MesageSettings.Frequency,
		SpreadingFactor: uplinkInfo.UplinkMessage.MesageSettings.DataRate.LORA.SpreadingFactor,
		Bandwidth:       uplinkInfo.UplinkMessage.MesageSettings.DataRate.LORA.Bandwidth,
		NumGateways:     len(uplinkInfo.UplinkMessage.GatewayMetadata),
		GatewayID:       firstGW.PacketBroker.ForwarderGatewayID,
		RSSI:            float64(firstGW.RSSI),
		SNR:             firstGW.SNR,
		TimeAtReception: uplinkInfo.ReceivedByGatewayAt,
	}
	report := toolbox.SubjectReportRequest{
		SubjectIP:       uplinkInfo.EndDeviceIDs.DeviceEUI,
		SubjectHostName: uplinkInfo.EndDeviceIDs.DeviceID,
		SubjectPlatform: uplinkInfo.EndDeviceIDs.ApplicationIDs.ApplicationID,
		SubjectComment:  comment,
	}

	if uplinkInfo.UplinkMessage.PortNumber == AppCommandPort && len(payloadBytes) > toolbox.MinPasswordLength+3 {
		// The port number matches the magic port number for transporting an app command. Ask store&forward message processor to execute it.
		report.CommandRequest.Command = string(payloadBytes)
	}
	cmdResp := hand.cmdProc.Features.MessageProcessor.StoreReport(r.Context(), report, uplinkInfo.EndDeviceIDs.DeviceID, "httpd")
	// At SF9/125kHz, the maximum payload size drops to 115 bytes.
	// At SF7/125kHz, the maximum payload size is about 222 bytes.
	// The LoRaWAN protocol takes away another ~13 bytes.
	// Reference: https://www.thethingsnetwork.org/forum/t/fair-use-policy-explained/1300
	// To be on the conservative side, limit the result length to SF9/125kHz's maximum.
	if result := cmdResp.CommandResponse.Result; len(result) > 100 {
		cmdResp.CommandResponse.Result = result[:110]
	}
	// Schedule a downlink message multiple times to transmit the app command execution result.
	if len(cmdResp.CommandResponse.Result) > 0 {
		authHeaderValue := "Bearer " + r.Header.Get("X-Downlink-Apikey")
		replaceEndpoint := r.Header.Get("X-Downlink-Replace")
		downlinkResp, err := inet.DoHTTP(r.Context(), inet.HTTPRequest{
			Method:      http.MethodPost,
			ContentType: "application/json",
			Header:      http.Header{"Authorization": []string{authHeaderValue}},
			Body: strings.NewReader(Downlinks{
				DownlinkMessage: []DownlinkMessage{
					{
						Port:             AppCommandPort,
						RawPayloadBase64: base64.StdEncoding.EncodeToString([]byte(cmdResp.CommandResponse.Result)),
						Priority:         DownlinkPriorityNormal,
					},
				},
			}.JSONString()),
		}, strings.Replace(replaceEndpoint, "%", "%%", -1))
		if err != nil {
			err = downlinkResp.Non2xxToError()
		}
		if err != nil {
			hand.logger.Warning("HandleTheThingsNetworkHTTPIntegration.Handler", middleware.GetRealClientIP(r), err, "failed to send downlink reply message")
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (_ *HandleTheThingsNetworkHTTPIntegration) GetRateLimitFactor() int {
	return 6
}

func (_ *HandleTheThingsNetworkHTTPIntegration) SelfTest() error {
	return nil
}
