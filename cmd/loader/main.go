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

	// For each paragraph, detect which characters appear
	for i := range paragraphs {
		paragraphs[i].Characters = characters.DetectCharacters(paragraphs[i].Text)
	}

	return paragraphs, nil
}

func main() {
	characters.InitRules()

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

		// 3. Create paragraph nodes and relationship links
		relationshipCount := 0
		for _, p := range paragraphs {
			_, err = tx.Run(ctx, "CREATE (p:Paragraph {number: $number, text: $text})", map[string]interface{}{
				"number": p.Number,
				"text":   p.Text,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create paragraph %d: %w", p.Number, err)
			}

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
		}

		fmt.Printf("Transaction prepared: loaded %d individuals, %d paragraphs, and %d relationships.\n", len(characters.Characters), len(paragraphs), relationshipCount)
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
			MATCH (p:Paragraph) WITH charCount, count(p) as paraCount
			MATCH ()-[r:APPEARED_IN]->() RETURN charCount, paraCount, count(r) as relCount
		`, nil)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			record := res.Record()
			charCount, _ := record.Get("charCount")
			paraCount, _ := record.Get("paraCount")
			relCount, _ := record.Get("relCount")
			fmt.Printf("Verification: found %v Individuals, %v Paragraphs, and %v APPEARED_IN relationships in database.\n", charCount, paraCount, relCount)
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
