# Alice in Wonderland Graph Browser

A Go-based application that parses Lewis Carroll's *Alice's Adventures in Wonderland*, extracts characters (individuals) appearing in each paragraph, constructs a graph in Neo4j, and serves a high-fidelity web application to browse character appearances and highlight their text occurrences.

## Project Structure

- `characters/`: Shared package containing character rules and entity extraction definitions.
- `cmd/loader/`: The data ingestion program that reads `AliceInWonderland.txt`, populates Neo4j with Paragraph and Individual nodes, links appearances, and verifies counts.
- `cmd/server/`: The web application that exposes REST APIs and hosts an embedded HTML/CSS/JS frontend browser interface.
- `static/`: Static resources (web templates).

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
- **Nodes**:
  - `Individual {name: String}`
  - `Paragraph {number: Integer, text: String}`
- **Relationships**:
  - `(:Individual)-[:APPEARED_IN]->(:Paragraph)`

## License
MIT License - see [LICENSE](LICENSE) for details.
