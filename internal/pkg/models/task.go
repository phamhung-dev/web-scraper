package models

import "github.com/google/uuid"

type Info map[string]interface{}
type LaunchConfiguration map[string]interface{}
type BehaviorActivity []map[string]interface{}
type MalwareConfigurationInfo []map[string]interface{}
type MalwareConfigurations []MalwareConfiguration
type TRiD []TRiDInfo
type EXIF []EXIFInfo
type URLs []string
type ProcessInformation map[string]interface{}
type ProcessInformations []ProcessInformation
type ModificationEvents []ModificationEvent
type DroppedFiles []DroppedFile
type HTTPOrHTTPSRequests []HTTPOrHTTPSRequest
type TCPOrUDPConnections []TCPOrUDPConnection
type IPs []string
type DNSRequests []DNSRequest
type ThreatConnections []ThreatConnection
type DebugOutputStrings []DebugOutputString

type General struct {
	Info                Info                `json:"info,omitempty"`
	LaunchConfiguration LaunchConfiguration `json:"launch_configuration,omitempty"`
}

type BehaviorActivities struct {
	Malicious  BehaviorActivity `json:"malicious,omitempty"`
	Suspicious BehaviorActivity `json:"suspicious,omitempty"`
	Info       BehaviorActivity `json:"info,omitempty"`
}

type MalwareConfiguration struct {
	FamilyName               string                   `json:"family_name,omitempty"`
	MalwareConfigurationInfo MalwareConfigurationInfo `json:"malware_configuration_info,omitempty"`
}

type StaticInformation struct {
	TRiD TRiD `json:"trid,omitempty"`
	EXIF EXIF `json:"exif,omitempty"`
}

type TRiDInfo struct {
	Ext   string `json:"ext,omitempty"`
	Value string `json:"value,omitempty"`
}

type EXIFInfo struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}
type VideoAndScreenshots struct {
	URLs URLs `json:"urls,omitempty"`
}

type Processes struct {
	Total               string              `json:"total,omitempty"`
	Monitored           string              `json:"monitored,omitempty"`
	Malicious           string              `json:"malicious,omitempty"`
	Suspicious          string              `json:"suspicious,omitempty"`
	ProcessInformations ProcessInformations `json:"process_informations,omitempty"`
}

type RegistryActivity struct {
	Total              string             `json:"total,omitempty"`
	Read               string             `json:"read,omitempty"`
	Write              string             `json:"write,omitempty"`
	Delete             string             `json:"delete,omitempty"`
	ModificationEvents ModificationEvents `json:"modification_events,omitempty"`
}

type ModificationEvent struct {
	PIDProcess string `json:"pid_process,omitempty"`
	Operation  string `json:"operation,omitempty"`
	Value      string `json:"value,omitempty"`
	Key        string `json:"key,omitempty"`
	Name       string `json:"name,omitempty"`
}

type FilesActivity struct {
	Executable   string       `json:"executable,omitempty"`
	Suspicious   string       `json:"suspicious,omitempty"`
	Text         string       `json:"text,omitempty"`
	Unknown      string       `json:"unknown,omitempty"`
	DroppedFiles DroppedFiles `json:"dropped_files,omitempty"`
}

type DroppedFile struct {
	PID      string `json:"pid,omitempty"`
	Process  string `json:"process,omitempty"`
	Filename string `json:"filename,omitempty"`
	Type     string `json:"type,omitempty"`
	MD5      string `json:"md5,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type NetworkActivity struct {
	HTTPOrHTTPS         string              `json:"http_or_https,omitempty"`
	TCPOrUDP            string              `json:"tcp_or_udp,omitempty"`
	DNS                 string              `json:"dns,omitempty"`
	Threats             string              `json:"threats,omitempty"`
	HTTPOrHTTPSRequests HTTPOrHTTPSRequests `json:"http_or_https_requests,omitempty"`
	TCPOrUDPConnections TCPOrUDPConnections `json:"tcp_or_udp_connections,omitempty"`
	DNSRequests         DNSRequests         `json:"dns_requests,omitempty"`
	ThreatConnections   ThreatConnections   `json:"threat_connections,omitempty"`
}

type HTTPOrHTTPSRequest struct {
	PID        string `json:"pid,omitempty"`
	Process    string `json:"process,omitempty"`
	Method     string `json:"method,omitempty"`
	HTTPCode   string `json:"http_code,omitempty"`
	IP         string `json:"ip,omitempty"`
	URL        string `json:"url,omitempty"`
	CN         string `json:"cn,omitempty"`
	Type       string `json:"type,omitempty"`
	Size       string `json:"size,omitempty"`
	Reputation string `json:"reputation,omitempty"`
}

type TCPOrUDPConnection struct {
	PID        string `json:"pid,omitempty"`
	Process    string `json:"process,omitempty"`
	IP         string `json:"ip,omitempty"`
	Domain     string `json:"domain,omitempty"`
	ASN        string `json:"asn,omitempty"`
	CN         string `json:"cn,omitempty"`
	Reputation string `json:"reputation,omitempty"`
}

type DNSRequest struct {
	Domain     string `json:"domain,omitempty"`
	IPs        IPs    `json:"ips,omitempty"`
	Reputation string `json:"reputation,omitempty"`
}

type ThreatConnection struct {
	PID     string `json:"pid,omitempty"`
	Process string `json:"process,omitempty"`
	Class   string `json:"class,omitempty"`
	Message string `json:"message,omitempty"`
}

type DebugOutputString struct {
	Process string `json:"process,omitempty"`
	Message string `json:"message,omitempty"`
}

type Task struct {
	ID                    uuid.UUID             `json:"id,omitempty"`
	General               General               `json:"general,omitempty"`
	BehaviorActivities    BehaviorActivities    `json:"behavior_activities,omitempty"`
	MalwareConfigurations MalwareConfigurations `json:"malware_configurations,omitempty"`
	StaticInformation     StaticInformation     `json:"static_information,omitempty"`
	VideoAndScreenshots   VideoAndScreenshots   `json:"video_and_screenshots,omitempty"`
	Processes             Processes             `json:"processes,omitempty"`
	RegistryActivity      RegistryActivity      `json:"registry_activity,omitempty"`
	FilesActivity         FilesActivity         `json:"files_activity,omitempty"`
	NetworkActivity       NetworkActivity       `json:"network_activity,omitempty"`
	DebugOutputStrings    DebugOutputStrings    `json:"debug_ouput_strings,omitempty"`
}
