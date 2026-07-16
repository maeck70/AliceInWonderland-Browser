# Alice in Wonderland Graph Browser (GoTH Stack)

A Go-based web application that parses Lewis Carroll's *Alice's Adventures in Wonderland*, extracts characters (individuals) and settings (locations) appearing in each paragraph, constructs an interactive graph in Neo4j, and serves a high-fidelity web application to browse appearances, cross-link visits, and highlight text occurrences.

This application is built using the **GoTH stack (Go, Templ, and HTMX)**, delivering a reactive Single Page Application (SPA) experience with server-side rendered HTML fragments and zero frontend JavaScript frameworks.

---

## The GoTH Stack Architecture

- **Go (Backend)**: Connects to Neo4j, queries nodes and relationships using Cypher, parses input files, and runs the web server.
- **Templ (Go Templates)**: A type-safe HTML templating engine for Go. Templates are written in `.templ` files and compiled into raw Go code, providing compile-time type safety for HTML structures, loop rendering, and argument bindings.
- **HTMX (Dynamic DOM Swapping)**: A lightweight JavaScript library loaded in the header that intercepts anchor clicks, form submissions, and input events, making AJAX requests directly from HTML attributes (e.g. `hx-get`, `hx-target`) and replacing specific DOM elements without page reloads.

---

## Project Structure

- `characters/`: Shared package containing character and location rules, entity extraction matchers, and regex rules.
- `cmd/loader/`: Data ingestion program that parses `AliceInWonderland.txt` and loads paragraphs, characters, locations, and relationships into Neo4j.
- `cmd/server/`: Main HTTP server that handles HTMX endpoints and serves template components.
- `cmd/server/templates/`: 
  - `components.templ`: The UI component source definitions (HTML layout, sidebar, list, detail views, and highlights).
  - `components_templ.go`: Compiled template files generated automatically by the `templ` compiler.

---

## Configuration

Configuration values are managed by **`godotenv`** and the Go standard **`flag`** module. On startup, the application attempts to read defaults from a tracked `.env` file at the root:

```env
NEO4J_URI=bolt://localhost:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=neo4jguest
HTTP_PORT=8080
```

### CLI Overrides
Any of the settings inside `.env` can be overridden on execution using command-line flags:
*   `-port`: Port to serve the web application (e.g., `-port 8085`)
*   `-uri`: Neo4j bolt connection URI
*   `-user`: Neo4j database username
*   `-password`: Neo4j database password

---

## How to Run

### 1. Ingest Data
Run the loader to parse the text and load it into Neo4j:
```bash
go run cmd/loader/main.go
```

### 2. Compile Templates (Go Templ)
Install the `templ` compiler command and generate the compiled Go template files (`*_templ.go`):
```bash
# 1. Install the templ compiler (one-time global installation)
go install github.com/a-h/templ/cmd/templ@latest

# 2. Compile templates inside your project workspace
templ generate
```

> [!TIP]
> **Developer Watch Mode**: During development, you can run `templ generate --watch` in a separate terminal. It will watch for edits inside `.templ` files and automatically recompile them in real-time.

### 3. Run the Web Server
Launch the server to serve the HTMX frontend browser interface:
```bash
go run cmd/server/main.go
```
Open [http://localhost:8080](http://localhost:8080) (or whichever port you specified in `.env` / CLI flags) in your web browser.

---

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

---

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

---

## License
MIT License - see [LICENSE](LICENSE) for details.
