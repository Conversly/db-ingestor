# Data ingestion service :

```jsx
           +-------------------+
           |   Document Source  |
           +-------------------+
                     |
                     v
              +--------------+
              |  Loader/Chunker| ( Eino loader, splitters )
              +--------------+
                     |
                     v
              +--------------+
              |  Queue/Buffer| ( current : in-memory queue/ future : kafka )
              +--------------+
                     |
          +----------+----------+
          |          |          |
          v          v          v
     Worker 1    Worker 2    Worker N.  (current : go routines/ future : workers )
          |          |          |
          v          v          v
        DB Write / Index / Further Processing

```

workers will embedd the chunks.  they will reveice array of all the chunks of 1 api request. 

Key points:

- **Producers**: Components that fetch or receive documents from sources (files, APIs, web, etc.).
- **Queue/Buffer**: Holds documents temporarily so multiple workers can process them asynchronously.
- **Workers**: Independently consume documents, parse them, and insert/process them for storage or embedding.
- **Database**: Central storage or index. Could be Postgres, MySQL, or a vector DB for embeddings.

## **2. Queue Options Without Kafka**

Since you want to avoid Kafka initially, here are lightweight options:

1. **In-memory Queue (for MVP / small scale)**
    - Simple Go channels (`chan Document`) can act as a queue.
    - Pros: Extremely simple, zero dependencies.
    - Cons: Not durable; if your process dies, everything in memory is lost.

## **3. Worker Design**

- Each worker should be **idempotent**: able to re-process a doc safely in case of failure.
- Process steps:
    1. Load document.
    2. Parse/transform content (CSV, PDF, DOCX, HTML, etc.).
    3. Optional: Generate embeddings (for AI search).
    4. Insert into DB.
    5. Ack completion (if using persistent queue).
- Limit concurrency per worker to avoid DB overload.