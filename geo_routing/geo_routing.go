package geo_routing
//
//
//Uhhhh i am not using this file right now
//
//
import (
	"fmt"
	"log"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// GetGeoLocation retrieves city and country from IP
func GetGeoLocation(ip string) (string, error) {
	db, err := geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		return "", fmt.Errorf("failed to open GeoLite2 database: %w", err)
	}
	defer db.Close()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	record, err := db.City(parsedIP)
	if err != nil {
		return "", fmt.Errorf("failed to lookup IP: %w", err)
	}

	country := record.Country.Names["en"]
	city := record.City.Names["en"]
	return fmt.Sprintf("%s, %s", city, country), nil
}

// DetermineNodeRegion identifies the best region for a new node
func DetermineNodeRegion(ip string) string {
	// Mock logic: Normally this would use a geolocation API
	log.Println("[GeoRouting] Determining region for IP:", ip)
	return "ASIA"
}
func main() {
	ip := "8.8.8.8" // Example IP (Google's public DNS)
	location, err := GetGeoLocation(ip)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Node Location:", location)
}
