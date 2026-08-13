"""shumbola.ai RAG mikroservisi.

agent-core (Go) uchun oddiy HTTP qatlam: query matnini
paraphrase-multilingual-MiniLM-L12-v2 bilan vektorlashtiradi va Chroma
vektor bazasidan (RAG bilim bazasi — TZ 8-bo'lim) eng mos chunk'larni
qaytaradi. Model va Chroma ulanishi jarayon boshlanganda BIR MARTA
yuklanadi (har so'rovda emas).
"""
import os
from contextlib import asynccontextmanager

import chromadb
from fastapi import FastAPI, HTTPException
from fastembed import TextEmbedding
from pydantic import BaseModel

CHROMA_DB_PATH = os.environ.get("CHROMA_DB_PATH", "/data/chroma_db")
COLLECTION_NAME = os.environ.get("CHROMA_COLLECTION", "shumbola_pentest_kb")
MODEL_NAME = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"

_embedding_model: TextEmbedding | None = None
_collection = None


@asynccontextmanager
async def lifespan(_app: FastAPI):
    global _embedding_model, _collection
    _embedding_model = TextEmbedding(model_name=MODEL_NAME)
    client = chromadb.PersistentClient(path=CHROMA_DB_PATH)
    _collection = client.get_collection(COLLECTION_NAME)
    yield


app = FastAPI(title="shumbola.ai RAG service", lifespan=lifespan)


class QueryRequest(BaseModel):
    query: str
    k: int = 5


class Chunk(BaseModel):
    text: str
    source: str
    file_path: str
    heading: str
    distance: float


class QueryResponse(BaseModel):
    chunks: list[Chunk]


@app.get("/healthz")
def healthz() -> dict:
    return {"status": "ok"}


@app.post("/query", response_model=QueryResponse)
def query(req: QueryRequest) -> QueryResponse:
    if not req.query.strip():
        raise HTTPException(status_code=400, detail="query must not be empty")
    if _embedding_model is None or _collection is None:
        raise HTTPException(status_code=503, detail="rag service not ready")

    k = req.k if req.k > 0 else 5
    q_emb = list(_embedding_model.embed([req.query]))[0].tolist()
    res = _collection.query(
        query_embeddings=[q_emb],
        n_results=k,
        include=["documents", "metadatas", "distances"],
    )

    chunks = []
    if res["ids"]:
        for i in range(len(res["ids"][0])):
            meta = res["metadatas"][0][i]
            chunks.append(
                Chunk(
                    text=res["documents"][0][i],
                    source=meta.get("source", ""),
                    file_path=meta.get("file_path", ""),
                    heading=meta.get("heading", ""),
                    distance=res["distances"][0][i],
                )
            )
    return QueryResponse(chunks=chunks)
