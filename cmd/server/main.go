package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"alice-neo4j/cmd/server/templates"
)

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found or error loading it, using system environment variables.")
	}

	dbURIDefault := getEnv("NEO4J_URI", "bolt://localhost:7687")
	dbUserDefault := getEnv("NEO4J_USER", "neo4j")
	dbPasswordDefault := getEnv("NEO4J_PASSWORD", "neo4jguest")

	portDefault := getEnv("HTTP_PORT", "8080")
	portFlag := flag.String("port", portDefault, "Port to serve the web application on")
	dbURIFlag := flag.String("uri", dbURIDefault, "Neo4j Database Bolt URI")
	dbUserFlag := flag.String("user", dbUserDefault, "Neo4j Database Username")
	dbPasswordFlag := flag.String("password", dbPasswordDefault, "Neo4j Database Password")
	flag.Parse()

	ctx := context.Background()

	fmt.Printf("Connecting to Neo4j at %s...\n", *dbURIFlag)
	driver, err := neo4j.NewDriverWithContext(*dbURIFlag, neo4j.BasicAuth(*dbUserFlag, *dbPasswordFlag, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}
	defer driver.Close(ctx)

	if err = driver.VerifyConnectivity(ctx); err != nil {
		log.Fatalf("Failed to verify Neo4j connectivity: %v", err)
	}
	fmt.Println("Connected to Neo4j successfully.")

	// HTML Render Handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		sidebarItems, err := getSidebarItems(ctx, session, "characters", "")
		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = templates.PageLayout("characters", sidebarItems).Render(r.Context(), w)
		if err != nil {
			log.Printf("Render error: %v", err)
		}
	})

	http.HandleFunc("/sidebar", func(w http.ResponseWriter, r *http.Request) {
		tab := r.URL.Query().Get("tab")
		if tab == "" {
			tab = "characters"
		}
		q := r.URL.Query().Get("q")

		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		sidebarItems, err := getSidebarItems(ctx, session, tab, q)
		if err != nil {
			http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = templates.SidebarList(tab, sidebarItems, "").Render(r.Context(), w)
		if err != nil {
			log.Printf("Render error: %v", err)
		}
	})

	http.HandleFunc("/details", func(w http.ResponseWriter, r *http.Request) {
		tab := r.URL.Query().Get("tab")
		name := r.URL.Query().Get("name")

		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		defer session.Close(ctx)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if tab == "characters" {
			visited, err := getCharacterLocations(ctx, session, name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			companions, err := getCharacterInteractions(ctx, session, name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			paragraphs, err := getCharacterParagraphs(ctx, session, name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			err = templates.CharacterDetails(name, len(paragraphs), visited, companions, paragraphs).Render(r.Context(), w)
			if err != nil {
				log.Printf("Render error: %v", err)
			}
		} else if tab == "locations" {
			visitors, err := getLocationCharacters(ctx, session, name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			paragraphs, err := getLocationParagraphs(ctx, session, name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			err = templates.LocationDetails(name, len(paragraphs), visitors, paragraphs).Render(r.Context(), w)
			if err != nil {
				log.Printf("Render error: %v", err)
			}
		} else if tab == "shared" {
			char1 := r.URL.Query().Get("char1")
			char2 := r.URL.Query().Get("char2")

			paragraphs, err := getSharedParagraphs(ctx, session, char1, char2)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			err = templates.SharedDetails(char1, char2, len(paragraphs), paragraphs).Render(r.Context(), w)
			if err != nil {
				log.Printf("Render error: %v", err)
			}
		} else {
			http.Error(w, "invalid or missing tab parameter", http.StatusBadRequest)
		}
	})

	fmt.Printf("Web server starting on http://localhost:%s...\n", *portFlag)
	log.Fatal(http.ListenAndServe(":"+*portFlag, nil))
}

func getSidebarItems(ctx context.Context, session neo4j.SessionWithContext, tab string, query string) ([]templates.SidebarItem, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		var cypherQuery string
		var params map[string]interface{}

		if tab == "characters" {
			if query != "" {
				cypherQuery = `
					MATCH (c:Individual)
					WHERE toLower(c.name) CONTAINS toLower($q)
					OPTIONAL MATCH (c)-[:APPEARED_IN]->(p:Paragraph)
					RETURN c.name as name, count(p) as appearances
					ORDER BY name ASC
				`
				params = map[string]interface{}{"q": query}
			} else {
				cypherQuery = `
					MATCH (c:Individual)
					OPTIONAL MATCH (c)-[:APPEARED_IN]->(p:Paragraph)
					RETURN c.name as name, count(p) as appearances
					ORDER BY name ASC
				`
			}
		} else {
			if query != "" {
				cypherQuery = `
					MATCH (l:Location)
					WHERE toLower(l.name) CONTAINS toLower($q)
					OPTIONAL MATCH (p:Paragraph)-[:LOCATED_IN]->(l)
					RETURN l.name as name, count(p) as appearances
					ORDER BY name ASC
				`
				params = map[string]interface{}{"q": query}
			} else {
				cypherQuery = `
					MATCH (l:Location)
					OPTIONAL MATCH (p:Paragraph)-[:LOCATED_IN]->(l)
					RETURN l.name as name, count(p) as appearances
					ORDER BY name ASC
				`
			}
		}

		res, err := tx.Run(ctx, cypherQuery, params)
		if err != nil {
			return nil, err
		}

		list := []templates.SidebarItem{}
		for res.Next(ctx) {
			rec := res.Record()
			nameVal, _ := rec.Get("name")
			appVal, _ := rec.Get("appearances")

			name, _ := nameVal.(string)
			app, _ := appVal.(int64)

			list = append(list, templates.SidebarItem{
				Name:        name,
				Appearances: app,
			})
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]templates.SidebarItem), nil
}

func getCharacterLocations(ctx context.Context, session neo4j.SessionWithContext, name string) ([]string, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (c:Individual {name: $name})-[:VISITED]->(l:Location)
			RETURN l.name as name
			ORDER BY name ASC
		`, map[string]interface{}{"name": name})
		if err != nil {
			return nil, err
		}
		list := []string{}
		for res.Next(ctx) {
			rec := res.Record()
			nameVal, _ := rec.Get("name")
			list = append(list, nameVal.(string))
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]string), nil
}

func getCharacterInteractions(ctx context.Context, session neo4j.SessionWithContext, name string) ([]string, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (c:Individual {name: $name})-[:MET_AT]->(other:Individual)
			RETURN other.name as name
			ORDER BY name ASC
		`, map[string]interface{}{"name": name})
		if err != nil {
			return nil, err
		}
		list := []string{}
		for res.Next(ctx) {
			rec := res.Record()
			nameVal, _ := rec.Get("name")
			list = append(list, nameVal.(string))
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]string), nil
}

func getCharacterParagraphs(ctx context.Context, session neo4j.SessionWithContext, name string) ([]templates.ParagraphItem, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (c:Individual {name: $name})-[:APPEARED_IN]->(p:Paragraph)
			RETURN p.number as number, p.text as text
			ORDER BY p.number ASC
		`, map[string]interface{}{"name": name})
		if err != nil {
			return nil, err
		}
		list := []templates.ParagraphItem{}
		for res.Next(ctx) {
			rec := res.Record()
			numVal, _ := rec.Get("number")
			textVal, _ := rec.Get("text")
			list = append(list, templates.ParagraphItem{
				Number: numVal.(int64),
				Text:   textVal.(string),
			})
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]templates.ParagraphItem), nil
}

func getLocationCharacters(ctx context.Context, session neo4j.SessionWithContext, name string) ([]string, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (c:Individual)-[:VISITED]->(l:Location {name: $name})
			RETURN c.name as name
			ORDER BY name ASC
		`, map[string]interface{}{"name": name})
		if err != nil {
			return nil, err
		}
		list := []string{}
		for res.Next(ctx) {
			rec := res.Record()
			nameVal, _ := rec.Get("name")
			list = append(list, nameVal.(string))
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]string), nil
}

func getLocationParagraphs(ctx context.Context, session neo4j.SessionWithContext, name string) ([]templates.ParagraphItem, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (p:Paragraph)-[:LOCATED_IN]->(l:Location {name: $name})
			RETURN p.number as number, p.text as text
			ORDER BY p.number ASC
		`, map[string]interface{}{"name": name})
		if err != nil {
			return nil, err
		}
		list := []templates.ParagraphItem{}
		for res.Next(ctx) {
			rec := res.Record()
			numVal, _ := rec.Get("number")
			textVal, _ := rec.Get("text")
			list = append(list, templates.ParagraphItem{
				Number: numVal.(int64),
				Text:   textVal.(string),
			})
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]templates.ParagraphItem), nil
}

func getSharedParagraphs(ctx context.Context, session neo4j.SessionWithContext, char1, char2 string) ([]templates.ParagraphItem, error) {
	resVal, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (cA:Individual {name: $char1})-[:APPEARED_IN]->(p:Paragraph)<-[:APPEARED_IN]-(cB:Individual {name: $char2})
			MATCH (p)-[:LOCATED_IN]->(l:Location)
			RETURN p.number as number, p.text as text, l.name as location
			ORDER BY p.number ASC
		`, map[string]interface{}{
			"char1": char1,
			"char2": char2,
		})
		if err != nil {
			return nil, err
		}
		list := []templates.ParagraphItem{}
		for res.Next(ctx) {
			rec := res.Record()
			numVal, _ := rec.Get("number")
			textVal, _ := rec.Get("text")
			locVal, _ := rec.Get("location")
			list = append(list, templates.ParagraphItem{
				Number:   numVal.(int64),
				Text:     textVal.(string),
				Location: locVal.(string),
			})
		}
		return list, nil
	})
	if err != nil {
		return nil, err
	}
	return resVal.([]templates.ParagraphItem), nil
}
