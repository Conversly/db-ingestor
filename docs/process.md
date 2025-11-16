req structure : 

```
{
  "userId": "string (required)",
  "chatbotId": "string (required)",
  "websiteUrls": ["string array (optional)"],
  "qandaData": [
    {
      "question": "string (required)",
      "answer": "string (required)",
      "metadata": "object (optional)"
    }
  ],
  "textContent": ["string array (optional)"],
  }
}
```

DATABASE TABLES : 

```
model embeddings {
  id           Int                   @id @default(autoincrement())
  userId       String                @db.VarChar
  chatbotid    String                // cuid2 string ID
  topic        String                @db.VarChar
  text         String                @db.VarChar
  embedding    Unsupported("vector")
  createdAt    DateTime?             @default(now()) @db.Timestamptz(6)
  updatedAt    DateTime?             @default(now()) @db.Timestamptz(6)
  dataSourceId Int?
  citation     String?
  ChatBot      ChatBot               @relation(fields: [chatbotid], references: [id], onDelete: Cascade, onUpdate: NoAction, map: "fk_chatbotid")
  DataSource   DataSource?           @relation(fields: [dataSourceId], references: [id], onDelete: Cascade, onUpdate: NoAction, map: "fk_datasourceid")

  @@index([embedding], map: "embedding_idx")
  @@index([citation], map: "idx_embeddings_citation")
}

model DataSource {
  id            Int          @id @default(autoincrement())
  chatbotId     String       // cuid2 string ID
  type          String
  sourceDetails Json         @db.Json
  createdAt     DateTime?    @default(now()) @db.Timestamptz(6)
  updatedAt     DateTime?    @default(now()) @db.Timestamptz(6)
  name          String       @db.VarChar
  citation      String?
  ChatBot       ChatBot      @relation(fields: [chatbotId], references: [id], onDelete: Cascade, onUpdate: NoAction, map: "fk_chatbot")
  embeddings    embeddings[]

  @@index([citation], map: "idx_datasource_citation")
}

```



1. use Eino website, document, loaders to parse the data
2. save the datasources in database.
2. use Eino splitter to chunk the data. 
3. prepare the data to send to a channel.  proper data structure.
4. embedding workers will consume data from the channel and embed the database. and save it in database. 
5. citation is name of datasource : 
citation : file name/ website url/ 'QnA'


output : 

{
  chatbotId
  datasource : website url/document name/QnA/
}