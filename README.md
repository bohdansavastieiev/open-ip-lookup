# Open IP Lookup

Open IP Lookup is a free aggregator of open-source information providers designed for bulk IP analysis.

The goal of this project is to provide a free and offline alternative to corporate tools for security analysis. It provides Geolocation, ASN data, and flags like VPN, proxy, cloud, etc, from logs, alerts, or any text input.

You can use the live site at [openiplookup.com](https://openiplookup.com), or run it entirely locally without sending any data to a third party during the lookup phase.

## Getting Started

To run the application locally using Docker:

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Add your MaxMind GeoLite2 credentials to `.env` (see below).
3. Start the application:

   ```bash
   docker compose up -d
   ```

4. Open `http://localhost:8080` in your browser.

## MaxMind GeoLite2 Dependency

This application requires the **MaxMind GeoLite2 City** dataset for geographic lookups, as it is currently the only supported geo data source.

You need a free MaxMind account to run the app:

1. Register at [MaxMind](https://www.maxmind.com/en/geolite2/signup).
2. Generate a license key.
3. Add your Account ID and License Key to your `.env` file.

## Data Sources

The application aggregates data from the following public sources. If you use this tool, you must comply with the upstream licenses and terms of use for these datasets.

| Source | Link | Upstream License / Terms |
| :--- | :--- | :--- |
| **MaxMind GeoLite2** | [City & ASN](https://www.maxmind.com/en/geolite2/eula) | Custom EULA |
| **IANA** | [IPv4](https://www.iana.org/assignments/iana-ipv4-special-registry), [IPv6](https://www.iana.org/assignments/iana-ipv6-special-registry) | Public Domain |
| **Team Cymru** | [IPv4 & IPv6 Full Bogons](https://www.team-cymru.com/bogons/) | Free to Use |
| **IPverse** | [AS IP Blocks](https://github.com/ipverse/as-ip-blocks), [Metadata](https://github.com/ipverse/as-metadata) | CC0 1.0 Universal |
| **Umkus IP Index** | [ASN Datacenters](https://github.com/Umkus/ip-index) | GPL-3.0 |
| **bountyyfi** | [Bad ASN List](https://github.com/bountyyfi/bad-asn-list) | MIT |
| **X4BNet** | [VPN & Datacenter IPv4](https://github.com/X4BNet/lists_vpn) | MIT |
| **az0** | [VPN IPs & Hostnames](https://github.com/az0/vpn_ip) | GPL-3.0 |
| **tobilg** | [Cloud Provider IPs](https://github.com/tobilg/public-cloud-provider-ip-ranges) | MIT |
| **rezmoss** | [Cloud Provider IPs](https://github.com/rezmoss/cloud-provider-ip-addresses) | CC0 1.0 Universal |
| **dan.me.uk** | [Tor Nodes](https://www.dan.me.uk/tornodes) | Terms of Use |
| **avastel** | [Bot IPs Lists](https://github.com/antoinevastel/avastel-bot-ips-lists) | MIT |

## License

The code in this repository is licensed under the MIT License. The data sources are governed by their own licenses as listed above.
