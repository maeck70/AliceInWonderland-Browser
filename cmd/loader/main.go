package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"alice-neo4j/characters"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Paragraph struct {
	Number     int
	Text       string
	Characters []string
	Locations  []string
}

func parseBook(filePath string) ([]Paragraph, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var paragraphs []Paragraph
	scanner := bufio.NewScanner(file)

	const (
		startMarker = "*** START OF THE PROJECT GUTENBERG EBOOK ALICE'S ADVENTURES IN WONDERLAND ***"
		endMarker   = "*** END OF THE PROJECT GUTENBERG EBOOK ALICE'S ADVENTURES IN WONDERLAND ***"
	)

	inStory := false
	var currentParaLines []string
	paraCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == startMarker {
			inStory = true
			continue
		}
		if trimmedLine == endMarker {
			inStory = false
			break
		}

		if !inStory {
			continue
		}

		if trimmedLine == "" {
			if len(currentParaLines) > 0 {
				paraText := strings.TrimSpace(strings.Join(currentParaLines, "\n"))
				if len(paraText) > 0 {
					paraCount++
					paragraphs = append(paragraphs, Paragraph{
						Number: paraCount,
						Text:   paraText,
					})
				}
				currentParaLines = nil
			}
		} else {
			currentParaLines = append(currentParaLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Flush remaining paragraph if any
	if len(currentParaLines) > 0 {
		paraText := strings.TrimSpace(strings.Join(currentParaLines, "\n"))
		if len(paraText) > 0 {
			paraCount++
			paragraphs = append(paragraphs, Paragraph{
				Number: paraCount,
				Text:   paraText,
			})
		}
	}

	// For each paragraph, detect which characters and locations appear
	for i := range paragraphs {
		paragraphs[i].Characters = characters.DetectCharacters(paragraphs[i].Text)
		paragraphs[i].Locations = characters.DetectLocations(paragraphs[i].Text)
	}

	return paragraphs, nil
}

func main() {
	characters.InitRules()
	characters.InitLocationRules()

	filePath := "AliceInWonderland.txt"
	fmt.Printf("Parsing book: %s...\n", filePath)
	paragraphs, err := parseBook(filePath)
	if err != nil {
		log.Fatalf("Error parsing book: %v", err)
	}
	fmt.Printf("Successfully parsed %d paragraphs from the book.\n", len(paragraphs))

	dbURI := getEnv("NEO4J_URI", "bolt://localhost:7687")
	dbUser := getEnv("NEO4J_USER", "neo4j")
	dbPassword := getEnv("NEO4J_PASSWORD", "neo4jguest")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Printf("Connecting to Neo4j at %s...\n", dbURI)
	driver, err := neo4j.NewDriverWithContext(dbURI, neo4j.BasicAuth(dbUser, dbPassword, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
	}
	defer driver.Close(ctx)

	if err = driver.VerifyConnectivity(ctx); err != nil {
		log.Fatalf("Failed to verify Neo4j connectivity: %v", err)
	}
	fmt.Println("Connected to Neo4j database successfully.")

	session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Create constraints first in separate transactions
	fmt.Println("Creating constraints...")
	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, "CREATE CONSTRAINT character_name_unique IF NOT EXISTS FOR (c:Individual) REQUIRE c.name IS UNIQUE", nil)
		return nil, err
	})
	if err != nil {
		fmt.Printf("Warning constraint Individual creation: %v\n", err)
	}

	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, "CREATE CONSTRAINT paragraph_number_unique IF NOT EXISTS FOR (p:Paragraph) REQUIRE p.number IS UNIQUE", nil)
		return nil, err
	})
	if err != nil {
		fmt.Printf("Warning constraint Paragraph creation: %v\n", err)
	}

	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, "CREATE CONSTRAINT location_name_unique IF NOT EXISTS FOR (l:Location) REQUIRE l.name IS UNIQUE", nil)
		return nil, err
	})
	if err != nil {
		fmt.Printf("Warning constraint Location creation: %v\n", err)
	}

	fmt.Println("Clearing database and inserting records in a single transaction...")
	startTime := time.Now()

	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		// 1. Clear database
		_, err := tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to clear DB: %w", err)
		}

		// 2. Create all character nodes
		for _, char := range characters.Characters {
			_, err = tx.Run(ctx, "CREATE (c:Individual {name: $name})", map[string]interface{}{"name": char.Name})
			if err != nil {
				return nil, fmt.Errorf("failed to create character %q: %w", char.Name, err)
			}
		}

		// 3. Create all location nodes
		for _, loc := range characters.Locations {
			_, err = tx.Run(ctx, "CREATE (l:Location {name: $name})", map[string]interface{}{"name": loc.Name})
			if err != nil {
				return nil, fmt.Errorf("failed to create location %q: %w", loc.Name, err)
			}
		}

		// 4. Create paragraph nodes and relationship links
		relationshipCount := 0
		locatedRelCount := 0
		visitedRelCount := 0
		metRelCount := 0
		for _, p := range paragraphs {
			_, err = tx.Run(ctx, "CREATE (p:Paragraph {number: $number, text: $text})", map[string]interface{}{
				"number": p.Number,
				"text":   p.Text,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create paragraph %d: %w", p.Number, err)
			}

			// Link characters to paragraph
			for _, charName := range p.Characters {
				_, err = tx.Run(ctx, `
					MATCH (c:Individual {name: $charName}), (p:Paragraph {number: $paraNum})
					CREATE (c)-[:APPEARED_IN]->(p)
				`, map[string]interface{}{
					"charName": charName,
					"paraNum":  p.Number,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to link character %q to paragraph %d: %w", charName, p.Number, err)
				}
				relationshipCount++
			}

			// Link paragraph to locations, and link characters to locations via VISITED
			for _, locName := range p.Locations {
				_, err = tx.Run(ctx, `
					MATCH (l:Location {name: $locName}), (p:Paragraph {number: $paraNum})
					CREATE (p)-[:LOCATED_IN]->(l)
				`, map[string]interface{}{
					"locName": locName,
					"paraNum": p.Number,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to link paragraph %d to location %q: %w", p.Number, locName, err)
				}
				locatedRelCount++

				for _, charName := range p.Characters {
					_, err = tx.Run(ctx, `
						MATCH (c:Individual {name: $charName}), (l:Location {name: $locName})
						MERGE (c)-[:VISITED]->(l)
					`, map[string]interface{}{
						"charName": charName,
						"locName":  locName,
					})
					if err != nil {
						return nil, fmt.Errorf("failed to link character %q to location %q: %w", charName, locName, err)
					}
					visitedRelCount++
				}
			}

			// Create MET_AT relationships between co-occurring characters in paragraphs that have locations
			if len(p.Locations) > 0 && len(p.Characters) > 1 {
				for i := 0; i < len(p.Characters); i++ {
					for j := i + 1; j < len(p.Characters); j++ {
						charA := p.Characters[i]
						charB := p.Characters[j]
						_, err = tx.Run(ctx, `
							MATCH (cA:Individual {name: $charA}), (cB:Individual {name: $charB})
							MERGE (cA)-[:MET_AT]->(cB)
							MERGE (cB)-[:MET_AT]->(cA)
						`, map[string]interface{}{
							"charA": charA,
							"charB": charB,
						})
						if err != nil {
							return nil, fmt.Errorf("failed to link character %q and %q with MET_AT: %w", charA, charB, err)
						}
						metRelCount += 2
					}
				}
			}
		}

		fmt.Printf("Transaction prepared: loaded %d individuals, %d locations, %d paragraphs, and:\n", len(characters.Characters), len(characters.Locations), len(paragraphs))
		fmt.Printf("  - %d APPEARED_IN relationships\n", relationshipCount)
		fmt.Printf("  - %d LOCATED_IN relationships\n", locatedRelCount)
		fmt.Printf("  - %d MET_AT relationships\n", metRelCount)
		return nil, nil
	})

	if err != nil {
		log.Fatalf("Failed to execute write transaction: %v", err)
	}

	fmt.Printf("Database load finished successfully in %v.\n", time.Since(startTime))

	// Verification query
	fmt.Println("\nVerifying database contents:")
	_, err = session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, `
			MATCH (c:Individual) WITH count(c) as charCount
			MATCH (l:Location) WITH charCount, count(l) as locCount
			MATCH (p:Paragraph) WITH charCount, locCount, count(p) as paraCount
			MATCH ()-[r1:APPEARED_IN]->() WITH charCount, locCount, paraCount, count(r1) as appCount
			MATCH ()-[r2:LOCATED_IN]->() WITH charCount, locCount, paraCount, appCount, count(r2) as locRelCount
			MATCH ()-[r3:VISITED]->() WITH charCount, locCount, paraCount, appCount, locRelCount, count(r3) as visitedCount
			MATCH ()-[r4:MET_AT]->() RETURN charCount, locCount, paraCount, appCount, locRelCount, visitedCount, count(r4) as metCount
		`, nil)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			record := res.Record()
			charCount, _ := record.Get("charCount")
			locCount, _ := record.Get("locCount")
			paraCount, _ := record.Get("paraCount")
			appCount, _ := record.Get("appCount")
			locRelCount, _ := record.Get("locRelCount")
			visitedCount, _ := record.Get("visitedCount")
			metCount, _ := record.Get("metCount")
			fmt.Printf("Verification: found %v Individuals, %v Locations, %v Paragraphs, and:\n", charCount, locCount, paraCount)
			fmt.Printf("  - %v APPEARED_IN relationships\n", appCount)
			fmt.Printf("  - %v LOCATED_IN relationships\n", locRelCount)
			fmt.Printf("  - %v VISITED relationships\n", visitedCount)
			fmt.Printf("  - %v MET_AT relationships\n", metCount)
		}
		return nil, nil
	})
	if err != nil {
		fmt.Printf("Warning verification query failed: %v\n", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
