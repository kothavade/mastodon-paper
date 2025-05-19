package collect_data

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/oschwald/maxminddb-golang"
)

const (
	StatusPending  = "pending"
	StatusChecking = "checking"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
)

const maxWorkers = 10

type geoInfo struct {
	CountryCode string
	ASN         uint
	ASName      string
}

// instanceStats holds the stats part of the Mastodon instance response
type instanceStats struct {
	Stats struct {
		UserCount   int `json:"user_count"`
		StatusCount int `json:"status_count"`
	} `json:"stats"`
}

// initInfoDB initializes the SQLite database with a table that contains node info
func initInfoDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./node_filter.db")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create nodes table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS node_info (
		domain       TEXT PRIMARY KEY,
		status       TEXT,
		ip           TEXT,
		asn          TEXT,
		country_code TEXT,
		user_count   INTEGER,
		post_count   INTEGER,
		cloud_provider TEXT,
		last_updated TIMESTAMP
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

// initializeInfoNodes inserts node info into the database if they don't exist
func initializeInfoNodes(db *sql.DB, nodes []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO node_info (domain, status, last_updated) 
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, node := range nodes {
		_, err := stmt.Exec(node, StatusPending)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func CollectData() {

	// Open filtered_nodes.json file
	nodes, err := os.ReadFile("filtered_processed_nodes.json")
	if err != nil {
		fmt.Println("Error reading nodes.json:", err)
		return
	}

	// Reading filtered_nodes.json
	var nodesList []string
	err = json.Unmarshal(nodes, &nodesList)
	if err != nil {
		fmt.Println("Error unmarshalling nodes.json:", err)
		return
	}

	// Init database
	db, err := initInfoDB()
	if err != nil {
		fmt.Println("Error opening nodes db", err)
	}
	defer db.Close()

	country_db_v4, err := maxminddb.Open("collect_data/country-ipv4.mmdb")
	if err != nil {
		fmt.Println("Error opening country_db_v4", err)
	}
	defer country_db_v4.Close()

	country_db_v6, err := maxminddb.Open("collect_data/country-ipv6.mmdb")
	if err != nil {
		fmt.Println("Error opening country_db_v6", err)
	}
	defer country_db_v6.Close()

	asn_db_v4, err := maxminddb.Open("collect_data/asn-ipv4.mmdb")
	if err != nil {
		fmt.Println("Error opening country_db_v4", err)
	}
	defer asn_db_v4.Close()

	asn_db_v6, err := maxminddb.Open("collect_data/asn-ipv6.mmdb")
	if err != nil {
		fmt.Println("Error opening country_db_v4", err)
	}
	defer asn_db_v6.Close()

	initializeInfoNodes(db, nodesList)

	client := &http.Client{Timeout: 5 * time.Second}

	node := nodesList[3]

	updateStatus(db, node, "in_progress")

	ip, err := lookupIP(node)
	if err != nil {
		updateStatus(db, node, StatusFailed)
		fmt.Println("Error looking up IP", err)
		return
	}

	version, err := IPVersion(ip)
	if err != nil {
		fmt.Println("Error looking up IP Version", err)
		updateStatus(db, node, StatusFailed)
	}
	var geo *geoInfo
	switch version {
	case "ipv4":
		geo, err = lookupGeo(ip, asn_db_v4, country_db_v4)
	case "ipv6":
		geo, err = lookupGeo(ip, asn_db_v6, country_db_v6)
	default:
		err = fmt.Errorf("unknown IP version")
	}
	if err != nil {
		fmt.Println("Error looking up Geo", err)
		updateStatus(db, node, StatusFailed)
	}

	// 4) Mastodon instance stats
	inst, err := fetchInstanceStats(node, client)
	if err != nil {
		fmt.Println("Error fetching Instance Stats", err)
		updateStatus(db, node, "failed")
		return
	}

	// 5) Write back as finished
	updateNodeInfo(db, node,
		ip,
		fmt.Sprintf("%d", geo.ASN),
		geo.CountryCode,
		inst.Stats.UserCount,
		inst.Stats.StatusCount,
		detectCloudProviderFromOrg(geo.ASName))
}

func lookupIP(domain string) (string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("DNS lookup failed: %v", err)
	}
	return ips[0].String(), nil
}

func lookupGeo(ipStr string, asn_reader *maxminddb.Reader, country_reader *maxminddb.Reader) (*geoInfo, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP %q", ipStr)
	}

	// 1) ASN lookup
	var asnRec struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	if err := asn_reader.Lookup(ip, &asnRec); err != nil {
		return nil, fmt.Errorf("ASN lookup failed: %w", err)
	}

	// 2) Country lookup
	var countryRec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := country_reader.Lookup(ip, &countryRec); err != nil {
		return nil, fmt.Errorf("Country lookup failed: %w", err)
	}
	fmt.Printf("got country code %q\n", countryRec.CountryCode)

	return &geoInfo{
		CountryCode: countryRec.CountryCode,
		ASN:         asnRec.AutonomousSystemNumber,
		ASName:      asnRec.AutonomousSystemOrganization,
	}, nil
}
func fetchInstanceStats(domain string, client *http.Client) (*instanceStats, error) {
	url := fmt.Sprintf("https://%s/api/v1/instance", domain)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("non-200 status: %d; body=%q", resp.StatusCode, string(body))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected content-type %q; body=%q", ct, string(body))
	}

	var stats instanceStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("JSON decode error: %w", err)
	}
	return &stats, nil
}

func detectCloudProviderFromOrg(org string) string {
	org = strings.ToLower(org)
	switch {
	case strings.Contains(org, "amazon"):
		return "AWS"
	case strings.Contains(org, "google"):
		return "GCP"
	case strings.Contains(org, "microsoft") || strings.Contains(org, "azure"):
		return "Azure"
	case strings.Contains(org, "digitalocean"):
		return "DigitalOcean"
	case strings.Contains(org, "linode"):
		return "Linode"
	case strings.Contains(org, "ovh"):
		return "OVH"
	case strings.Contains(org, "hetzner"):
		return "Hetzner"
	default:
		return ""
	}
}

func updateStatus(db *sql.DB, domain, status string) {
	_, _ = db.Exec(
		`UPDATE node_info SET status=?, last_updated=CURRENT_TIMESTAMP WHERE domain=?`,
		status, domain,
	)
}

func updateNodeInfo(db *sql.DB, domain, ip, asn, country string, userCount, postCount int, cloud string) {
	_, _ = db.Exec(
		`UPDATE node_info SET
            status=?,
            ip=?, asn=?, country_code=?,
            user_count=?, post_count=?, cloud_provider=?,
            last_updated=CURRENT_TIMESTAMP
         WHERE domain=?`, StatusSuccess,
		ip, asn, country, userCount, postCount, cloud, domain,
	)
}

func IPVersion(ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", errors.New("invalid IP format")
	}
	if ip.To4() != nil {
		return "ipv4", nil
	}
	return "ipv6", nil
}
