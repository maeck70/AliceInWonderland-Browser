# Alice in Wonderland Graph Browser

A Go-based application that parses Lewis Carroll's *Alice's Adventures in Wonderland*, extracts characters (individuals) and settings (locations) appearing in each paragraph, constructs a graph in Neo4j, and serves a high-fidelity web application to browse appearances, cross-link visits, and highlight text occurrences.

## Project Structure

- `characters/`: Shared package containing character and location rules, entity extraction matchers, and regex rules.
- `cmd/loader/`: The data ingestion program that reads `AliceInWonderland.txt`, populates Neo4j with Paragraph, Individual, and Location nodes, constructs relationships, and verifies counts.
- `cmd/server/`: The web application that exposes REST APIs and hosts an embedded HTML/CSS/JS frontend browser interface.
- `cmd/server/static/`: Embedded frontend templates (HTML/CSS/JS).

## Database Configuration

By default, the application connects to a local Neo4j database on Podman/Docker:
- **Bolt Port**: `7687` (HTTP browser on `7474`)
- **Username**: `neo4j`
- **Password**: `neo4jguest`

You can override these by setting the following environment variables:
- `NEO4J_URI`
- `NEO4J_USER`
- `NEO4J_PASSWORD`

## How to Run

### 1. Ingest Data
Run the loader to parse the text and load it into Neo4j:
```bash
go run cmd/loader/main.go
```

### 2. Run the Web Server
Launch the server to serve the frontend browser interface:
```bash
go run cmd/server/main.go -port 8080
```
Open [http://localhost:8080](http://localhost:8080) in your web browser.

## Graph Schema

### Nodes
- `Individual {name: String}` (unique name constraint)
- `Location {name: String}` (unique name constraint)
- `Paragraph {number: Integer, text: String}` (unique number constraint)

### Relationships
- `(:Individual)-[:APPEARED_IN]->(:Paragraph)`: Character is mentioned in the paragraph.
- `(:Paragraph)-[:LOCATED_IN]->(:Location)`: Paragraph takes place at the location.
- `(:Individual)-[:VISITED]->(:Location)`: Character visited the location (derived via paragraph occurrences).
- `(:Individual)-[:MET_AT]->(:Individual)`: Character met another character in the same setting/location (derived via paragraph occurrences).

## Useful Cypher Queries

Use the [Neo4j Browser](http://localhost:7474) (credentials `neo4j` / `neo4jguest`) to query the data:

*   **View Everything with Relationships**:
    ```cypher
    MATCH (n)
    OPTIONAL MATCH (n)-[r]->(m)
    RETURN n, r, m
    ```

*   **Show Connected Characters and Locations**:
    ```cypher
    MATCH (c:Individual)-[r:VISITED]->(l:Location)
    RETURN c, r, l
    ```

*   **Show All Met Connections (Character Interactions at Settings)**:
    ```cypher
    MATCH (c1:Individual)-[r:MET_AT]->(c2:Individual)
    RETURN c1, r, c2
    ```

*   **List Top 10 Characters by Appearance Count**:
    ```cypher
    MATCH (c:Individual)
    OPTIONAL MATCH (c)-[:APPEARED_IN]->(p:Paragraph)
    RETURN c.name as Character, count(p) as Appearances
    ORDER BY Appearances DESC
    LIMIT 10
    ```

*   **Queen of Hearts Locations & Interacting Characters**:
    Finds settings where the Queen of Hearts is present and lists the other characters she interacts with (co-occurs in paragraphs) sorted by frequency of interaction:
    ```cypher
    MATCH (queen:Individual {name: "Queen of Hearts"})-[:APPEARED_IN]->(p:Paragraph)
    MATCH (other:Individual)-[:APPEARED_IN]->(p)
    OPTIONAL MATCH (p)-[:LOCATED_IN]->(l:Location)
    WHERE other <> queen
    RETURN coalesce(l.name, "Unknown/Generic") as Location,
           collect(distinct other.name) as InteractedCharacters,
           count(distinct p) as CoOccurrencesCount
    ORDER BY CoOccurrencesCount DESC
    ```

## License
MIT License - see [LICENSE](LICENSE) for details.
