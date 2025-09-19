package common

// DataStruct represents the structure of the IP data returned by the API.
type DataStruct struct {
	IP       *string `json:"ip"`
	Hostname *string `json:"hostname"`
	Org      *string `json:"org"`
	City     *string `json:"city"`
	Region   *string `json:"region"`
	Country  *string `json:"country"`
	Timezone *string `json:"timezone"`
	Loc      *string `json:"loc"`
}

// ASNDataResponse represents the structure of the ASN data returned by the API.
type ASNDataResponse struct {
	Details  ASNDetails    `json:"details"`
	Prefixes ASNPrefixInfo `json:"prefixes"`
}

// ASNDetails represents the structure of the ASN details returned by the API.
type ASNDetails struct {
	ASN  uint   `json:"asn"`
	Name string `json:"name"`
}

// ASNPrefixInfo represents the structure of the ASN prefix information returned by the API.
type ASNPrefixInfo struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// DomainDataResponse represents the structure of the domain data returned by the API.
type DomainDataResponse struct {
	Whois interface{} `json:"whois"`
	DNS   DNSData     `json:"dns"`
}

// DNSData represents the structure of the DNS records.
type DNSData struct {
	A     []string `json:"A,omitempty"`
	AAAA  []string `json:"AAAA,omitempty"`
	CNAME string   `json:"CNAME,omitempty"`
	MX    []string `json:"MX,omitempty"`
	TXT   []string `json:"TXT,omitempty"`
	NS    []string `json:"NS,omitempty"`
	SOA   []string `json:"SOA,omitempty"`
	CAA   []string `json:"CAA,omitempty"`
}

// WhoisInfo is a sanitized version of the parsed whois data for the API response.
type WhoisInfo struct {
	Domain     *WhoisDomain    `json:"domain,omitempty"`
	Registrar  *WhoisRegistrar `json:"registrar,omitempty"`
	Registrant *WhoisContact   `json:"registrant,omitempty"`
	Admin      *WhoisContact   `json:"admin,omitempty"`
	Tech       *WhoisContact   `json:"tech,omitempty"`
}

// WhoisDomain omits unnecessary fields from the original parsed domain struct.
type WhoisDomain struct {
	ID             string   `json:"id,omitempty"`
	Domain         string   `json:"domain,omitempty"`
	WhoisServer    string   `json:"whois_server,omitempty"`
	Status         []string `json:"status,omitempty"`
	NameServers    []string `json:"name_servers,omitempty"`
	DNSSEC         bool     `json:"dnssec"`
	CreatedDate    string   `json:"created_date,omitempty"`
	UpdatedDate    string   `json:"updated_date,omitempty"`
	ExpirationDate string   `json:"expiration_date,omitempty"`
}

// WhoisRegistrar contains registrar information.
type WhoisRegistrar struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	ReferralURL string `json:"referral_url,omitempty"`
}

// WhoisContact contains contact information for registrant, admin, or tech.
type WhoisContact struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name,omitempty"`
	Organization string `json:"organization,omitempty"`
	Street       string `json:"street,omitempty"`
	City         string `json:"city,omitempty"`
	Province     string `json:"province,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
	Country      string `json:"country,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Fax          string `json:"fax,omitempty"`
	Email        string `json:"email,omitempty"`
}
