package lookup

// IPData represents the structure of the IP data returned by the API.
type IPData struct {
	IP       *string `json:"ip"`
	Hostname *string `json:"hostname"`
	Org      *string `json:"org"`
	City     *string `json:"city"`
	Region   *string `json:"region"`
	Country  *string `json:"country"`
	Timezone *string `json:"timezone"`
	Loc      *string `json:"loc"`
}

// ASNData represents the structure of the ASN data returned by the API.
type ASNData struct {
	Details  ASNDetails `json:"details"`
	Prefixes PrefixInfo `json:"prefixes"`
}

// ASNDetails represents the structure of the ASN details returned by the API.
type ASNDetails struct {
	ASN  uint   `json:"asn"`
	Name string `json:"name"`
}

// PrefixInfo represents the IPv4 and IPv6 prefixes associated with an ASN.
type PrefixInfo struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

// DomainData represents the WHOIS and DNS data for a domain.
type DomainData struct {
	Whois *WhoisInfo `json:"whois"`
	DNS   DNSData    `json:"dns"`
}

// DNSData represents the common DNS records resolved for a domain.
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

// WhoisInfo represents formatted WHOIS information.
type WhoisInfo struct {
	Domain     *WhoisDomain    `json:"domain,omitempty"`
	Registrar  *WhoisRegistrar `json:"registrar,omitempty"`
	Registrant *WhoisContact   `json:"registrant,omitempty"`
	Admin      *WhoisContact   `json:"admin,omitempty"`
	Tech       *WhoisContact   `json:"tech,omitempty"`
}

// WhoisDomain represents domain-level WHOIS information.
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

// WhoisRegistrar contains registrar contact information.
type WhoisRegistrar struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	Phone       string `json:"phone,omitempty"`
	ReferralURL string `json:"referral_url,omitempty"`
}

// WhoisContact contains individual contact details.
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

// BogonData represents the response returned when querying a bogon IP.
type BogonData struct {
	IP    string `json:"ip"`
	Bogon bool   `json:"bogon"`
}

// StringPtr returns a pointer to the given string, or nil if the string is empty.
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
