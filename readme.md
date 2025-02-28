# ipinfo

`ipinfo` is a powerful and efficient IP information service written in Go. It fetches GeoIP data to provide detailed information about an IP address, including geographical location, ASN, and related network details. The service automatically updates its GeoIP databases to ensure accuracy and reliability.

## Features

- **IP Geolocation**: Provides city, region, country, continent, and coordinates for any IP address.
- **ASN Information**: Includes autonomous system number and organization.
- **Hostname Lookup**: Retrieves the hostname associated with the IP address.
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
