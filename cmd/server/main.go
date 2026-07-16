package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

//go:embed static/index.html
var staticFiles embed.FS

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func main() {
	portFlag := flag.String("port", "8080", "Port to serve the web application on")
	flag.Parse()

	indexHTML, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		log.Fatalf("Failed to read embedded static/index.html: %v", err)
	}

	dbURI := getEnv("NEO4J_URI", "bolt://localhost:7687")
	dbUser := getEnv("NEO4J_USER", "neo4j")
	dbPassword := getEnv("NEO4J_PASSWORD", "neo4jguest")

	ctx := context.Background()

	fmt.Printf("Connecting to Neo4j at %s...\n", dbURI)
	driver, err := neo4j.NewDriverWithContext(dbURI, neo4j.BasicAuth(dbUser, dbPassword, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}
	defer driver.Close(ctx)

	if err = driver.VerifyConnectivity(ctx); err != nil {
		log.Fatalf("Failed to verify Neo4j connectivity: %v", err)
	}
	fmt.Println("Connected to Neo4j successfully.")

	// HTTP Routing
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	http.HandleFunc("/api/characters", func(w http.ResponseWriter, r *http.Request) {
		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			res, err := tx.Run(ctx, `
				MATCH (c:Individual)
				OPTIONAL MATCH (c)-[:APPEARED_IN]->(p:Paragraph)
				RETURN c.name as name, count(p) as appearances
				ORDER BY name ASC
			`, nil)
			if err != nil {
				return nil, err
			}

			type CharAppearance struct {
				Name        string `json:"name"`
				Appearances int64  `json:"appearances"`
			}
			var list []CharAppearance

			for res.Next(ctx) {
				rec := res.Record()
				nameVal, _ := rec.Get("name")
				appVal, _ := rec.Get("appearances")

				name, _ := nameVal.(string)
				app, _ := appVal.(int64)

				list = append(list, CharAppearance{
					Name:        name,
					Appearances: app,
				})
			}
			return list, nil
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/api/locations", func(w http.ResponseWriter, r *http.Request) {
		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			res, err := tx.Run(ctx, `
				MATCH (l:Location)
				OPTIONAL MATCH (p:Paragraph)-[:LOCATED_IN]->(l)
				RETURN l.name as name, count(p) as appearances
				ORDER BY name ASC
			`, nil)
			if err != nil {
				return nil, err
			}

			type LocAppearance struct {
				Name        string `json:"name"`
				Appearances int64  `json:"appearances"`
			}
			var list []LocAppearance

			for res.Next(ctx) {
				rec := res.Record()
				nameVal, _ := rec.Get("name")
				appVal, _ := rec.Get("appearances")

				name, _ := nameVal.(string)
				app, _ := appVal.(int64)

				list = append(list, LocAppearance{
					Name:        name,
					Appearances: app,
				})
			}
			return list, nil
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/api/paragraphs", func(w http.ResponseWriter, r *http.Request) {
		charName := r.URL.Query().Get("character")
		locName := r.URL.Query().Get("location")

		if charName == "" && locName == "" {
			http.Error(w, "missing 'character' or 'location' query parameter", http.StatusBadRequest)
			return
		}

		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			var query string
			var params map[string]interface{}

			if charName != "" {
				query = `
					MATCH (c:Individual {name: $name})-[:APPEARED_IN]->(p:Paragraph)
					RETURN p.number as number, p.text as text
					ORDER BY p.number ASC
				`
				params = map[string]interface{}{"name": charName}
			} else {
				query = `
					MATCH (p:Paragraph)-[:LOCATED_IN]->(l:Location {name: $name})
					RETURN p.number as number, p.text as text
					ORDER BY p.number ASC
				`
				params = map[string]interface{}{"name": locName}
			}

			res, err := tx.Run(ctx, query, params)
			if err != nil {
				return nil, err
			}

			type ParagraphResult struct {
				Number int64  `json:"number"`
				Text   string `json:"text"`
			}
			var list []ParagraphResult

			for res.Next(ctx) {
				rec := res.Record()
				numVal, _ := rec.Get("number")
				textVal, _ := rec.Get("text")

				num, _ := numVal.(int64)
				text, _ := textVal.(string)

				list = append(list, ParagraphResult{
					Number: num,
					Text:   text,
				})
			}
			return list, nil
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/api/character-locations", func(w http.ResponseWriter, r *http.Request) {
		charName := r.URL.Query().Get("character")
		if charName == "" {
			http.Error(w, "missing 'character' query parameter", http.StatusBadRequest)
			return
		}

		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			res, err := tx.Run(ctx, `
				MATCH (c:Individual {name: $name})-[:VISITED]->(l:Location)
				RETURN l.name as name
				ORDER BY name ASC
			`, map[string]interface{}{"name": charName})
			if err != nil {
				return nil, err
			}

			var list []string
			for res.Next(ctx) {
				rec := res.Record()
				nameVal, _ := rec.Get("name")
				name, _ := nameVal.(string)
				list = append(list, name)
			}
			return list, nil
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	http.HandleFunc("/api/location-characters", func(w http.ResponseWriter, r *http.Request) {
		locName := r.URL.Query().Get("location")
		if locName == "" {
			http.Error(w, "missing 'location' query parameter", http.StatusBadRequest)
			return
		}

		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			res, err := tx.Run(ctx, `
				MATCH (c:Individual)-[:VISITED]->(l:Location {name: $name})
				RETURN c.name as name
				ORDER BY name ASC
			`, map[string]interface{}{"name": locName})
			if err != nil {
				return nil, err
			}

			var list []string
			for res.Next(ctx) {
				rec := res.Record()
				nameVal, _ := rec.Get("name")
				name, _ := nameVal.(string)
				list = append(list, name)
			}
			return list, nil
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	fmt.Printf("Web server starting on http://localhost:%s...\n", *portFlag)
	log.Fatal(http.ListenAndServe(":"+*portFlag, nil))
}
