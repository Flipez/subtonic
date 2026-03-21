package sonos

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Device represents a Sonos speaker or group that can be controlled.
type Device struct {
	Name                  string
	IP                    string
	AVTransportURL        string
	RenderingControlURL   string
	// Set for multi-room groups: the coordinator's RINCON UUID and the
	// AVTransport URLs of non-coordinator members.
	CoordinatorUUID       string
	MemberAVTransportURLs []string
}

// Client controls a single Sonos speaker via UPnP/SOAP.
type Client struct {
	device Device
}

// New returns a Client for the given device.
func New(device Device) *Client {
	return &Client{device: device}
}

// DeviceName returns the friendly name of the device.
func (c *Client) DeviceName() string {
	return c.device.Name
}

// Discover sends SSDP M-SEARCH, fetches device descriptions, then queries
// ZoneGroupTopology so that groups (multi-room) are shown as a single entry
// whose commands go to the group coordinator.
func Discover(timeout time.Duration) ([]Device, error) {
	locations, err := ssdpSearch(timeout)
	if err != nil {
		return nil, err
	}
	if len(locations) == 0 {
		return nil, nil
	}

	// Fetch device description for each discovered IP.
	devicesByIP := make(map[string]Device, len(locations))
	for ip, loc := range locations {
		dev, err := fetchDeviceDescription(loc, ip)
		if err != nil {
			continue
		}
		devicesByIP[ip] = dev
	}
	if len(devicesByIP) == 0 {
		return nil, nil
	}

	// Query ZoneGroupTopology from any reachable device to get groups.
	for _, dev := range devicesByIP {
		groups, err := queryZoneGroups(baseURL(dev.AVTransportURL))
		if err != nil {
			continue
		}
		result := buildGroupDevices(groups, devicesByIP)
		if len(result) > 0 {
			return result, nil
		}
	}

	// Fall back to individual devices if group query fails.
	result := make([]Device, 0, len(devicesByIP))
	for _, d := range devicesByIP {
		result = append(result, d)
	}
	return result, nil
}

// ssdpSearch sends M-SEARCH and returns a map of IP → LOCATION URL.
func ssdpSearch(timeout time.Duration) (map[string]string, error) {
	ssdpAddr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return nil, fmt.Errorf("resolve ssdp addr: %w", err)
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}
	defer conn.Close()

	search := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 3\r\n" +
		"ST: urn:schemas-upnp-org:device:ZonePlayer:1\r\n\r\n"

	if _, err := conn.WriteToUDP([]byte(search), ssdpAddr); err != nil {
		return nil, fmt.Errorf("send m-search: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	locations := make(map[string]string)
	buf := make([]byte, 4096)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		ip := src.IP.String()
		if locations[ip] != "" {
			continue
		}
		if loc := extractHeader(string(buf[:n]), "location"); loc != "" {
			locations[ip] = loc
		}
	}
	return locations, nil
}

// zoneGroup holds raw data from ZoneGroupTopology.
type zoneGroup struct {
	CoordinatorUUID string
	Members         []zoneMember
}

type zoneMember struct {
	UUID     string
	Location string // device description URL, e.g. http://192.168.1.x:1400/xml/device_description.xml
	ZoneName string
}

// queryZoneGroups calls ZoneGroupTopology#GetZoneGroupState on the given base
// URL (e.g. "http://192.168.1.10:1400") and returns the parsed group list.
func queryZoneGroups(base string) ([]zoneGroup, error) {
	const zgtService = "urn:schemas-upnp-org:service:ZoneGroupTopology:1"
	zgtURL := base + "/ZoneGroupTopology/Control"
	body := fmt.Sprintf(`<u:GetZoneGroupState xmlns:u=%q></u:GetZoneGroupState>`, zgtService)

	data, err := soapCall(zgtURL, zgtService, "GetZoneGroupState", body)
	if err != nil {
		return nil, err
	}

	// Outer SOAP envelope: extract ZoneGroupState string (may be HTML-escaped XML).
	var outer struct {
		Body struct {
			Response struct {
				ZoneGroupState string `xml:"ZoneGroupState"`
			} `xml:"GetZoneGroupStateResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &outer); err != nil {
		return nil, fmt.Errorf("parse ZoneGroupState envelope: %w", err)
	}
	stateXML := strings.TrimSpace(outer.Body.Response.ZoneGroupState)
	if stateXML == "" {
		return nil, fmt.Errorf("empty ZoneGroupState")
	}
	// The value is HTML-escaped XML; unescape it.
	stateXML = html.UnescapeString(stateXML)

	// The unescaped value is <ZoneGroupState><ZoneGroups><ZoneGroup .../>...
	// so we need one extra level of nesting in our struct.
	var zoneGroups struct {
		ZoneGroups struct {
			Groups []struct {
				Coordinator string `xml:"Coordinator,attr"`
				Members     []struct {
					UUID     string `xml:"UUID,attr"`
					Location string `xml:"Location,attr"`
					ZoneName string `xml:"ZoneName,attr"`
				} `xml:"ZoneGroupMember"`
			} `xml:"ZoneGroup"`
		} `xml:"ZoneGroups"`
	}
	if err := xml.Unmarshal([]byte(stateXML), &zoneGroups); err != nil {
		return nil, fmt.Errorf("parse ZoneGroups XML: %w", err)
	}

	result := make([]zoneGroup, 0, len(zoneGroups.ZoneGroups.Groups))
	for _, g := range zoneGroups.ZoneGroups.Groups {
		members := make([]zoneMember, 0, len(g.Members))
		for _, m := range g.Members {
			members = append(members, zoneMember{
				UUID:     m.UUID,
				Location: m.Location,
				ZoneName: m.ZoneName,
			})
		}
		result = append(result, zoneGroup{
			CoordinatorUUID: g.Coordinator,
			Members:         members,
		})
	}
	return result, nil
}

// buildGroupDevices maps ZoneGroupTopology groups to Devices by matching
// coordinator IPs against the already-fetched devicesByIP map.
func buildGroupDevices(groups []zoneGroup, devicesByIP map[string]Device) []Device {
	// Build UUID → IP mapping from the member Location URLs.
	uuidToIP := make(map[string]string)
	for _, g := range groups {
		for _, m := range g.Members {
			ip := ipFromLocation(m.Location)
			if ip != "" {
				uuidToIP[m.UUID] = ip
			}
		}
	}

	var result []Device
	for _, g := range groups {
		// Find coordinator's IP.
		coordIP := uuidToIP[g.CoordinatorUUID]
		if coordIP == "" {
			continue
		}
		coordDev, ok := devicesByIP[coordIP]
		if !ok {
			// Coordinator not in our discovery map — try fetching it now.
			locURL := ""
			for _, m := range g.Members {
				if m.UUID == g.CoordinatorUUID {
					locURL = m.Location
					break
				}
			}
			if locURL == "" {
				continue
			}
			dev, err := fetchDeviceDescription(locURL, coordIP)
			if err != nil {
				continue
			}
			coordDev = dev
		}

		// Build a display name and collect non-coordinator member AVTransport URLs.
		names := make([]string, 0, len(g.Members))
		var memberURLs []string
		for _, m := range g.Members {
			if m.ZoneName != "" {
				names = append(names, m.ZoneName)
			}
			if m.UUID != g.CoordinatorUUID {
				memberIP := ipFromLocation(m.Location)
				if memberDev, ok2 := devicesByIP[memberIP]; ok2 {
					memberURLs = append(memberURLs, memberDev.AVTransportURL)
				}
			}
		}

		coordDev.Name = strings.Join(names, " + ")
		coordDev.CoordinatorUUID = g.CoordinatorUUID
		coordDev.MemberAVTransportURLs = memberURLs
		result = append(result, coordDev)
	}
	return result
}

// ipFromLocation extracts the host IP from a UPnP Location URL like
// "http://192.168.1.10:1400/xml/device_description.xml".
func ipFromLocation(loc string) string {
	if idx := strings.Index(loc, "://"); idx >= 0 {
		rest := loc[idx+3:]
		// rest is "192.168.1.10:1400/..." or "192.168.1.10/..."
		if colon := strings.IndexAny(rest, ":/"); colon >= 0 {
			return rest[:colon]
		}
		return rest
	}
	return ""
}

// extractHeader finds a header value (case-insensitive) in an HTTP-style
// response string (SSDP uses "\r\n"-separated headers).
func extractHeader(response, name string) string {
	nameLower := strings.ToLower(name) + ":"
	for _, line := range strings.Split(response, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), nameLower) {
			return strings.TrimSpace(line[len(nameLower):])
		}
	}
	return ""
}

// fetchDeviceDescription fetches the UPnP XML device description and extracts
// the friendly name and AVTransport / RenderingControl service URLs.
func fetchDeviceDescription(location, ip string) (Device, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(location) //nolint:noctx
	if err != nil {
		return Device{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Device{}, err
	}

	// Strip default XML namespace declarations so Go's encoding/xml can match
	// field names without namespace qualification (Sonos uses xmlns="...").
	body = []byte(strings.ReplaceAll(string(body), ` xmlns=`, ` _xmlns=`))

	// Sonos nests AVTransport and RenderingControl inside a MediaRenderer
	// sub-device under <deviceList>, so we need a recursive device structure.
	type upnpService struct {
		ServiceType string `xml:"serviceType"`
		ControlURL  string `xml:"controlURL"`
	}
	type upnpDevice struct {
		FriendlyName string `xml:"friendlyName"`
		ServiceList  struct {
			Services []upnpService `xml:"service"`
		} `xml:"serviceList"`
		DeviceList struct {
			Devices []upnpDevice `xml:"device"`
		} `xml:"deviceList"`
	}
	var root struct {
		Device upnpDevice `xml:"device"`
	}

	if err := xml.Unmarshal(body, &root); err != nil {
		return Device{}, fmt.Errorf("parse device xml: %w", err)
	}

	dev := Device{
		Name: root.Device.FriendlyName,
		IP:   ip,
	}

	// Derive the base URL (scheme + host) from the location URL.
	base := baseURL(location)

	// Walk the entire device tree to find AVTransport and RenderingControl,
	// which Sonos places in a nested MediaRenderer sub-device.
	var walk func(d upnpDevice)
	walk = func(d upnpDevice) {
		for _, svc := range d.ServiceList.Services {
			switch {
			case strings.Contains(svc.ServiceType, "AVTransport") && dev.AVTransportURL == "":
				dev.AVTransportURL = base + svc.ControlURL
			case strings.Contains(svc.ServiceType, "RenderingControl") && dev.RenderingControlURL == "":
				dev.RenderingControlURL = base + svc.ControlURL
			}
		}
		for _, sub := range d.DeviceList.Devices {
			walk(sub)
		}
	}
	walk(root.Device)

	if dev.AVTransportURL == "" || dev.RenderingControlURL == "" {
		return Device{}, fmt.Errorf("device %q missing required service URLs (avt=%q rcs=%q)",
			dev.Name, dev.AVTransportURL, dev.RenderingControlURL)
	}

	return dev, nil
}

// baseURL returns "scheme://host" from a full URL string.
func baseURL(u string) string {
	if idx := strings.Index(u, "://"); idx >= 0 {
		rest := u[idx+3:]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return u[:idx+3+slash]
		}
		return u
	}
	return ""
}

// --- SOAP helpers ---

const (
	avtService = "urn:schemas-upnp-org:service:AVTransport:1"
	rcsService = "urn:schemas-upnp-org:service:RenderingControl:1"
)

// soapCall performs a SOAP HTTP POST to url. It returns a non-nil error for
// both network failures and UPnP SOAP faults (HTTP 500 + <s:Fault> body).
func soapCall(url, service, action, bodyXML string) ([]byte, error) {
	envelope := `<?xml version="1.0" encoding="utf-8"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" ` +
		`s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body>` + bodyXML + `</s:Body>` +
		`</s:Envelope>`

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(envelope))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, service, action))

	hc := &http.Client{Timeout: 10 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// UPnP SOAP faults come back as HTTP 500 with a <s:Fault> body.
	if resp.StatusCode == http.StatusInternalServerError {
		return nil, parseSoapFault(data)
	}
	return data, nil
}

// parseSoapFault extracts a human-readable error from a SOAP fault body.
func parseSoapFault(data []byte) error {
	// Strip namespace prefixes for easier matching.
	cleaned := []byte(strings.ReplaceAll(string(data), ` xmlns=`, ` _xmlns=`))
	var fault struct {
		Body struct {
			Fault struct {
				FaultString string `xml:"faultstring"`
				Detail      struct {
					UPnPError struct {
						ErrorCode        int    `xml:"errorCode"`
						ErrorDescription string `xml:"errorDescription"`
					} `xml:"UPnPError"`
				} `xml:"detail"`
			} `xml:"Fault"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(cleaned, &fault); err == nil {
		code := fault.Body.Fault.Detail.UPnPError.ErrorCode
		desc := fault.Body.Fault.Detail.UPnPError.ErrorDescription
		fs := fault.Body.Fault.FaultString
		if code != 0 {
			return fmt.Errorf("UPnP error %d: %s (%s)", code, desc, fs)
		}
		if fs != "" {
			return fmt.Errorf("SOAP fault: %s", fs)
		}
	}
	return fmt.Errorf("SOAP fault (HTTP 500): %s", strings.TrimSpace(string(data)))
}

// --- AVTransport methods ---

// SetURI sets the transport URI with DIDL-Lite metadata so Sonos knows the
// MIME type without probing the stream (absence of metadata causes error 714).
//
// For grouped speakers: each non-coordinator member is first told to follow the
// coordinator via x-rincon:UUID. Some firmware versions drop group membership
// when the coordinator's transport URI changes; re-asserting it here keeps all
// speakers in sync.
func (c *Client) SetURI(streamURL, title, artist, album, suffix string) error {
	// Re-establish group membership for all non-coordinator members.
	if c.device.CoordinatorUUID != "" && len(c.device.MemberAVTransportURLs) > 0 {
		rinconURI := "x-rincon:" + c.device.CoordinatorUUID
		followBody := fmt.Sprintf(
			`<u:SetAVTransportURI xmlns:u=%q>`+
				`<InstanceID>0</InstanceID>`+
				`<CurrentURI>%s</CurrentURI>`+
				`<CurrentURIMetaData></CurrentURIMetaData>`+
				`</u:SetAVTransportURI>`,
			avtService, xmlEscape(rinconURI),
		)
		for _, memberURL := range c.device.MemberAVTransportURLs {
			soapCall(memberURL, avtService, "SetAVTransportURI", followBody) //nolint:errcheck
		}
	}

	// Build DIDL-Lite metadata and set the stream URI on the coordinator.
	mime := mimeFromSuffix(suffix)
	didl := fmt.Sprintf(
		`<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" `+
			`xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" `+
			`xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">`+
			`<item id="1" parentID="-1" restricted="true">`+
			`<dc:title>%s</dc:title>`+
			`<dc:creator>%s</dc:creator>`+
			`<upnp:album>%s</upnp:album>`+
			`<upnp:class>object.item.audioItem.musicTrack</upnp:class>`+
			`<res protocolInfo="http-get:*:%s:*">%s</res>`+
			`</item>`+
			`</DIDL-Lite>`,
		xmlEscape(title), xmlEscape(artist), xmlEscape(album), mime, xmlEscape(streamURL),
	)
	body := fmt.Sprintf(
		`<u:SetAVTransportURI xmlns:u=%q>`+
			`<InstanceID>0</InstanceID>`+
			`<CurrentURI>%s</CurrentURI>`+
			`<CurrentURIMetaData>%s</CurrentURIMetaData>`+
			`</u:SetAVTransportURI>`,
		avtService, xmlEscape(streamURL), xmlEscape(didl),
	)
	_, err := soapCall(c.device.AVTransportURL, avtService, "SetAVTransportURI", body)
	return err
}

// mimeFromSuffix maps a file extension to an audio MIME type for DIDL-Lite metadata.
func mimeFromSuffix(suffix string) string {
	switch strings.ToLower(suffix) {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "ogg", "oga", "opus":
		return "audio/ogg"
	case "aac":
		return "audio/aac"
	case "m4a", "mp4":
		return "audio/mp4"
	default:
		return "audio/mpeg"
	}
}

// Seek seeks the current track to pos using REL_TIME mode.
func (c *Client) Seek(pos time.Duration) error {
	h := int(pos.Hours())
	m := int(pos.Minutes()) % 60
	s := int(pos.Seconds()) % 60
	target := fmt.Sprintf("%d:%02d:%02d", h, m, s)
	body := fmt.Sprintf(
		`<u:Seek xmlns:u=%q>`+
			`<InstanceID>0</InstanceID>`+
			`<Unit>REL_TIME</Unit>`+
			`<Target>%s</Target>`+
			`</u:Seek>`,
		avtService, target,
	)
	_, err := soapCall(c.device.AVTransportURL, avtService, "Seek", body)
	return err
}

// Play starts or resumes playback.
func (c *Client) Play() error {
	body := fmt.Sprintf(
		`<u:Play xmlns:u=%q><InstanceID>0</InstanceID><Speed>1</Speed></u:Play>`,
		avtService,
	)
	_, err := soapCall(c.device.AVTransportURL, avtService, "Play", body)
	return err
}

// Pause pauses playback.
func (c *Client) Pause() error {
	body := fmt.Sprintf(
		`<u:Pause xmlns:u=%q><InstanceID>0</InstanceID></u:Pause>`,
		avtService,
	)
	_, err := soapCall(c.device.AVTransportURL, avtService, "Pause", body)
	return err
}

// Stop stops playback.
func (c *Client) Stop() error {
	body := fmt.Sprintf(
		`<u:Stop xmlns:u=%q><InstanceID>0</InstanceID></u:Stop>`,
		avtService,
	)
	_, err := soapCall(c.device.AVTransportURL, avtService, "Stop", body)
	return err
}

// GetPositionInfo returns the current playback position and track duration.
func (c *Client) GetPositionInfo() (pos, dur time.Duration, err error) {
	body := fmt.Sprintf(
		`<u:GetPositionInfo xmlns:u=%q><InstanceID>0</InstanceID></u:GetPositionInfo>`,
		avtService,
	)
	data, err := soapCall(c.device.AVTransportURL, avtService, "GetPositionInfo", body)
	if err != nil {
		return 0, 0, err
	}

	var result struct {
		Body struct {
			Response struct {
				RelTime       string `xml:"RelTime"`
				TrackDuration string `xml:"TrackDuration"`
			} `xml:"GetPositionInfoResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &result); err != nil {
		return 0, 0, fmt.Errorf("parse position info: %w", err)
	}

	pos = parseUPnPDuration(result.Body.Response.RelTime)
	dur = parseUPnPDuration(result.Body.Response.TrackDuration)
	return pos, dur, nil
}

// GetTransportInfo returns the current transport state string, e.g.
// "PLAYING", "PAUSED_PLAYBACK", or "STOPPED".
func (c *Client) GetTransportInfo() (string, error) {
	body := fmt.Sprintf(
		`<u:GetTransportInfo xmlns:u=%q><InstanceID>0</InstanceID></u:GetTransportInfo>`,
		avtService,
	)
	data, err := soapCall(c.device.AVTransportURL, avtService, "GetTransportInfo", body)
	if err != nil {
		return "", err
	}

	var result struct {
		Body struct {
			Response struct {
				CurrentTransportState string `xml:"CurrentTransportState"`
			} `xml:"GetTransportInfoResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse transport info: %w", err)
	}
	return result.Body.Response.CurrentTransportState, nil
}

// --- RenderingControl methods ---

// GetVolume returns the current master volume (0–100).
func (c *Client) GetVolume() (int, error) {
	body := fmt.Sprintf(
		`<u:GetVolume xmlns:u=%q>`+
			`<InstanceID>0</InstanceID>`+
			`<Channel>Master</Channel>`+
			`</u:GetVolume>`,
		rcsService,
	)
	data, err := soapCall(c.device.RenderingControlURL, rcsService, "GetVolume", body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Body struct {
			Response struct {
				CurrentVolume int `xml:"CurrentVolume"`
			} `xml:"GetVolumeResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(data, &result); err != nil {
		return 0, fmt.Errorf("parse volume: %w", err)
	}
	return result.Body.Response.CurrentVolume, nil
}

// SetVolume sets the master volume (0–100).
func (c *Client) SetVolume(vol int) error {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	body := fmt.Sprintf(
		`<u:SetVolume xmlns:u=%q>`+
			`<InstanceID>0</InstanceID>`+
			`<Channel>Master</Channel>`+
			`<DesiredVolume>%d</DesiredVolume>`+
			`</u:SetVolume>`,
		rcsService, vol,
	)
	_, err := soapCall(c.device.RenderingControlURL, rcsService, "SetVolume", body)
	return err
}

// --- helpers ---

// parseUPnPDuration parses a UPnP time string "H:MM:SS" into a time.Duration.
func parseUPnPDuration(s string) time.Duration {
	if s == "" || s == "NOT_IMPLEMENTED" {
		return 0
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	var h, m, sec int
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)
	fmt.Sscanf(parts[2], "%d", &sec)
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

// xmlEscape replaces XML-special characters with their entity equivalents.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
