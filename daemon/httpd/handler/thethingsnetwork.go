package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// UplinkMessageMetadataGateway describes a gateway that received an uplink message.
type UplinkMessageMetadataGateway struct {
	ID        string  `json:"gtw_id"`
	Timestamp int     `json:"timestamp"`
	Time      string  `json:"time"`
	Channel   int     `json:"channel"`
	RSSI      float64 `json:"rssi"`
	SNR       float64 `json:"snr"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

// UplinkMessageMetadata is the metadata part of an unlink message that describes the transmission and recipient quality.
type UplinkMessageMetadata struct {
	Time                     string                         `json:"time"`
	Frequency                float64                        `json:"frequency"`
	Modulation               string                         `json:"modulation"`
	SpreadingFactorBandwidth string                         `json:"data_rate"`
	BitRate                  float64                        `json:"bit_rate"`
	CodingRate               string                         `json:"coding_rate"`
	Gateways                 []UplinkMessageMetadataGateway `json:"gateways"`
	Latitude                 float64                        `json:"latitude"`
	Longitude                float64                        `json:"longitude"`
	Altitude                 float64                        `json:"altitude"`
}

// TTNMapperPayload is TTN-Mapper compatible payload fields embedded into an uplink message.
type TTNMapperPayload struct {
	Altitude  float64 `json:"altitude"`
	HDOP      float64 `json:"hdop"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// UplinkMessage is an uplink, TTN-Mapper compatible message transmitted by LoRA device, arrived via TTN HTTP integration.
type UplinkMessage struct {
	AppID            string                `json:"app_id"`
	DeviceID         string                `json:"dev_id"`
	DeviceEUISerial  string                `json:"hardware_serial"`
	Port             int                   `json:"port"`
	Counter          int                   `json:"counter"`
	RawPayloadBase64 string                `json:"payload_raw"`
	TTNMapperPayload TTNMapperPayload      `json:"payload_fields"`
	Metadata         UplinkMessageMetadata `json:"metadata"`
	DownlinkURL      string                `json:"downlink_url"`
}

// ReceptionComment describes a reception of TTN packet/message, the description describes the transmitter and gateway, and will be
// stored by store&forward message processor in-memory.
type ReceptionComment struct {
	DeviceID                            string
	UplinkSequenceNum                   int
	UplinkPort                          int
	Latitude, Longitude, Altitude, HDOP float64
	Frequency                           float64
	Modulation                          string
	SpreadingFactorBandwidth            string
	CodingRate                          string
	NumGateway                          int
	GatewayID                           string
	GWLatitude, GWLongitude, GWAltitude float64
	RSSI                                float64
	SNR                                 float64
	Channel                             int
	TimeAtReception                     string
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

// DownlinkMessage is made in reply to an UplinkMessage and will be schedule for transmission to LoRA device by a gateway.
type DownlinkMessage struct {
	DeviceID         string `json:"dev_id"`
	Port             int    `json:"port"`
	Confirmed        bool   `json:"confirmed"`
	RawPayloadBase64 string `json:"payload_raw"`
}

func (msg DownlinkMessage) ToJSONString() string {
	b, err := json.Marshal(msg)
	if err != nil {
		lalog.DefaultLogger.Warning("DownlinkMessage.ToJSONString", "", err, "failed to marshal message")
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
	var msg UplinkMessage
	if err := json.Unmarshal(body, &msg); err != nil || msg.AppID == "" || msg.DeviceID == "" {
		http.Error(w, "failed to decode uplink message", http.StatusInternalServerError)
		return
	}
	// Decode the raw payload sent by transmitter
	payloadBytes, err := base64.StdEncoding.DecodeString(msg.RawPayloadBase64)
	if err != nil {
		http.Error(w, "failed to decode uplink message payload", http.StatusInternalServerError)
		return
	}
	// Construct a report to save in message processor
	var firstGW UplinkMessageMetadataGateway
	if len(msg.Metadata.Gateways) > 0 {
		firstGW = msg.Metadata.Gateways[0]
	}
	hand.logger.Info("Handle", msg.DeviceEUISerial, nil,
		"received transmission from device %s, packet #%d on port %d, located at %f, %f (TTN Mapper %f, %f), received by gateway %s located at %f %f, payload size %d bytes.",
		msg.DeviceID, msg.Counter, msg.Port, msg.Metadata.Latitude, msg.Metadata.Longitude,
		msg.TTNMapperPayload.Latitude, msg.TTNMapperPayload.Longitude,
		firstGW.ID, firstGW.Latitude, firstGW.Longitude, len(payloadBytes))

	comment := ReceptionComment{
		DeviceID:                 msg.DeviceID,
		UplinkSequenceNum:        msg.Counter,
		UplinkPort:               msg.Port,
		Latitude:                 msg.TTNMapperPayload.Latitude,
		Longitude:                msg.TTNMapperPayload.Longitude,
		Altitude:                 msg.TTNMapperPayload.Altitude,
		HDOP:                     msg.TTNMapperPayload.HDOP,
		Frequency:                msg.Metadata.Frequency,
		Modulation:               msg.Metadata.Modulation,
		SpreadingFactorBandwidth: msg.Metadata.SpreadingFactorBandwidth,
		CodingRate:               msg.Metadata.CodingRate,
		NumGateway:               len(msg.Metadata.Gateways),
		GatewayID:                firstGW.ID,
		GWLatitude:               firstGW.Latitude,
		GWLongitude:              firstGW.Longitude,
		GWAltitude:               firstGW.Altitude,
		RSSI:                     firstGW.RSSI,
		SNR:                      firstGW.SNR,
		Channel:                  firstGW.Channel,
		TimeAtReception:          firstGW.Time,
	}
	report := toolbox.SubjectReportRequest{
		SubjectIP:       msg.DeviceEUISerial,
		SubjectHostName: msg.DeviceID,
		SubjectPlatform: msg.AppID,
		SubjectComment:  comment,
	}
	/*
		The first 10 bytes are decoded like this:
		(from https://github.com/kizniche/ttgo-tbeam-ttn-tracker)
		function Decoder(bytes, port) {
				var decoded = {};
				decoded.latitude = ((bytes[0]<<16)>>>0) + ((bytes[1]<<8)>>>0) + bytes[2];
				decoded.latitude = (decoded.latitude / 16777215.0 * 180) - 90;
				decoded.longitude = ((bytes[3]<<16)>>>0) + ((bytes[4]<<8)>>>0) + bytes[5];
				decoded.longitude = (decoded.longitude / 16777215.0 * 360) - 180;
				var altValue = ((bytes[6]<<8)>>>0) + bytes[7];
				var sign = bytes[6] & (1 << 7);
				if(sign) decoded.altitude = 0xFFFF0000 | altValue;
				else decoded.altitude = altValue;
				decoded.hdop = bytes[8] / 10.0;
				decoded.sats = bytes[9];
				return decoded;
		}
		After the 10th byte there comes the app command.
	*/
	if len(payloadBytes) > 10 {
		// There is an app command carried in the payload, ask store&forward message processor to execute it.
		report.CommandRequest.Command = strings.TrimSpace(string(bytes.TrimLeft(bytes.TrimRight(payloadBytes[10:], "\x00"), "\x00")))
	}
	cmdResp := hand.cmdProc.Features.MessageProcessor.StoreReport(r.Context(), report, msg.DeviceEUISerial, "httpd")
	/*
		Assume that LoRAWAN transmitter operates at SF8/125kHz (or better), at which the maximum payload size is 133 bytes across all regions.
		Among the payload, TTN uses "at least 13 bytes" for its own overhead.
		References:
		- https://docs.exploratory.engineering/lora/dr_sf/
		- https://www.thethingsnetwork.org/forum/t/limitations-data-rate-packet-size-30-seconds-uplink-and-10-messages-downlink-per-day-fair-access-policy-guidelines/1300
		Therefore, limit the downlink payload to 110 bytes, leaving 10 bytes of buffer just in case.
		Limiting command result size is usually carried out with LintText, but in this case with TTN there is an application constraint.
		Make sure the downstream message never exceeds 110 bytes, otherwise the LoRA transceiver may not get anything back.
	*/
	if result := cmdResp.CommandResponse.Result; len(result) > 110 {
		cmdResp.CommandResponse.Result = result[:110]
	}
	// Reply with app command execution result
	if len(report.CommandRequest.Command) > 10 {
		downlinkResp, err := inet.DoHTTP(r.Context(), inet.HTTPRequest{
			Method:      http.MethodPost,
			ContentType: "application/json",
			Body: strings.NewReader(DownlinkMessage{
				DeviceID:         msg.DeviceID,
				Port:             msg.Port,
				Confirmed:        false,
				RawPayloadBase64: base64.StdEncoding.EncodeToString([]byte(cmdResp.CommandResponse.Result)),
			}.ToJSONString()),
		}, strings.Replace(msg.DownlinkURL, "%", "%%", -1))
		if err != nil {
			err = downlinkResp.Non2xxToError()
		}
		if err != nil {
			hand.logger.Warning("HandleTheThingsNetworkHTTPIntegration.Handler", GetRealClientIP(r), err, "failed to send downlink reply message")
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
