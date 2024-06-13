# ipinfo
`ipinfo` is a powerful and efficient IP information service written in Go. It fetches GeoIP data to provide detailed information about an IP address, including geographical location, ASN, and related network details. The service automatically updates its GeoIP databases to ensure accuracy and reliability.

## Features
- **IP Geolocation**: Provides city, region, country, continent, and coordinates for any IP address.
- **ASN Information**: Includes autonomous system number and organization.
- **Hostname Lookup**: Retrieves the hostname associated with the IP address.
- **Automatic Database Updates**: Keeps GeoIP databases up-to-date weekly.
- **JSONP Support**: Allows JSONP responses for cross-domain requests.

## Example Endpoints
### Get information about an IP address
```sh
$ curl https://ip.albert.lol/8.8.8.8
{
  "ip": "8.8.8.8",
  "hostname": "dns.google.",
  "asn": "15169",
  "organization": "Google LLC",
  "city": "Mountain View",
  "region": "California",
  "country": "US",
  "country_full": "United States",
  "continent": "NA",
  "continent_full": "North America",
  "loc": "37.4223,-122.0850"
}
```
### Get specific information (e.g., city) about an IP address
```sh
$ curl https://ip.albert.lol/8.8.8.8/city
Mountain View
```
### Use JSONP callback function
```sh
$ curl http://ip.albert.lol/8.8.8.8?callback=getGoogle
/**/ typeof getGoogle === 'function' && getGoogle({
  "ip": "8.8.8.8",
  "hostname": "dns.google.",
  "asn": "15169",
  "organization": "Google LLC",
  "city": "Mountain View",
  "region": "California",
  "country": "US",
  "country_full": "United States",
  "continent": "NA",
  "continent_full": "North America",
  "loc": "37.4223,-122.0850"
});
```
```html
<script>
let getGoogle = function(data) {
  alert("Google's ASN is " + data.asn);
}
</script>
<script src="https://ip.albert.lol/8.8.8.8?callback=getGoogle"></script>
```

## Running Locally
### With Docker
```sh
git clone https://github.com/skidoodle/ipinfo
cd ipinfo
docker build -t ipinfo:main .
docker run -p 3000:3000 ipinfo:main
```
### Without Docker
```sh
git clone https://github.com/skidoodle/ipinfo
cd ipinfo
go run main.go
```

## Deploying
### Docker Compose
```yaml
version: '3.9'

services:
  ipinfo:
    container_name: ipinfo
    image: 'ghcr.io/skidoodle/ipinfo:main'
    restart: unless-stopped
    ports:
      - '3000:3000'
    volumes:
      - data:/app

volumes:
  data:
    driver: local
```
### Docker Run
```sh
docker run \
  -d \
  --name=ipinfo \
  --restart=unless-stopped \
  -p 3000:3000 \
  ghcr.io/skidoodle/ipinfo:main
```

## LICENSE
[GPL-3.0](https://github.com/skidoodle/ipinfo/blob/main/license)
