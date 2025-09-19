# ipinfo

`ipinfo` is a powerful and efficient IP information service written in Go. It fetches GeoIP data to provide detailed information about an IP address, including geographical location, ASN, and related network details. The service automatically updates its GeoIP databases to ensure accuracy and reliability.

## Features

- **IP Geolocation**: Provides city, region, country, continent, and coordinates for any IP address.
- **ASN Information**: Includes autonomous system number and organization.
- **Hostname Lookup**: Retrieves the hostname associated with the IP address.
- **Domain WHOIS**: Fetches structured WHOIS data for any domain.
- **Domain DNS Records**: Retrieves common DNS records (A, AAAA, CNAME, MX, TXT, NS).
- **Automatic Database Updates**: Keeps GeoIP databases up-to-date daily.

## Example Endpoints

### Get information about an IP address

```sh
$ curl https://ip.albert.lol/9.9.9.9
{
  "ip": "9.9.9.9",
  "hostname": "dns9.quad9.net",
  "asn": "19281",
  "org": "QUAD9-AS-1",
  "city": "Berkeley",
  "region": "California",
  "country": "United States",
  "continent": "North America",
  "timezone": "America/Los_Angeles",
  "loc": "37.8767,-122.2676"
}
```

### Get specific information (e.g., city) about an IP address

```sh
$ curl https://ip.albert.lol/9.9.9.9/city
{
  "city": "Berkeley"
}
```

### Get details about an ASN
```sh
$ curl https://ip.albert.lol/AS19281
{
  "details": {
    "asn": 19281,
    "name": "QUAD9-AS-1"
  },
  "prefixes": {
    "ipv4": [
      "149.112.112.0/24",
      "149.112.149.0/24",
      "199.249.255.0/24",
      "9.9.9.0/24"
    ],
    "ipv6": [
      "2001:0:909:900::/56",
      "2001:0:9570:7000::/56",
      "2001:0:9570:9500::/56",
      "2001:0:c7f9:ff00::/56",
      "2002:909:900::/40",
      "2002:9570:7000::/40",
      "2002:9570:9500::/40",
      "2002:c7f9:ff00::/40",
      "2620:fe::/48",
      "::909:900/120",
      "::9570:7000/120",
      "::9570:9500/120",
      "::c7f9:ff00/120"
    ]
  }
}
```

### Get WHOIS and DNS records for a domain
```sh
$ curl https://ip.albert.lol/example.com
{
  "whois": {
    "domain": {
      "id": "2336799_DOMAIN_COM-VRSN",
      "domain": "example.com",
      "whois_server": "whois.iana.org",
      "status": [
        "clientDeleteProhibited",
        "clientTransferProhibited",
        "clientUpdateProhibited"
      ],
      "name_servers": [
        "a.iana-servers.net",
        "b.iana-servers.net"
      ],
      "dnssec": true,
      "created_date": "1995-08-14T04:00:00Z",
      "updated_date": "2025-08-14T07:01:39Z",
      "expiration_date": "2026-08-13T04:00:00Z"
    }
  },
  "dns": {
    "A": [
      "93.184.216.34"
    ],
    "AAAA": [
      "2606:2800:220:1:248:1893:25c8:1946"
    ],
    "NS": [
      "a.iana-servers.net",
      "b.iana-servers.net"
    ]
  }
}
```

## Running Locally

### With Docker

```sh
git clone https://github.com/skidoodle/ipinfo
cd ipinfo
docker build -t ipinfo:main .
docker run \
-p 3000:3000
-e GEOIPUPDATE_ACCOUNT_ID=${GEOIPUPDATE_ACCOUNT_ID} \
-e GEOIPUPDATE_LICENSE_KEY=${GEOIPUPDATE_LICENSE_KEY} \
ipinfo:main
```

### Without Docker

```sh
git clone https://github.com/skidoodle/ipinfo
cd ipinfo
go run .
```

## Deploying

### Docker Compose

```yaml
services:
  ipinfo:
    image: ghcr.io/skidoodle/ipinfo:main
    container_name: ipinfo
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      GEOIPUPDATE_ACCOUNT_ID: ${GEOIPUPDATE_ACCOUNT_ID}
      GEOIPUPDATE_LICENSE_KEY: ${GEOIPUPDATE_LICENSE_KEY}
```

### Docker Run

```sh
docker run \
  -d \
  --name=ipinfo \
  --restart=unless-stopped \
  -p 3000:3000 \
  -e GEOIPUPDATE_ACCOUNT_ID=${GEOIPUPDATE_ACCOUNT_ID} \
  -e GEOIPUPDATE_LICENSE_KEY=${GEOIPUPDATE_LICENSE_KEY} \
  ghcr.io/skidoodle/ipinfo:main
```

## LICENSE

[GPL-3.0](https://github.com/skidoodle/ipinfo/blob/main/license)
