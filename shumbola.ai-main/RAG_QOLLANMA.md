# shumbola.ai RAG bazasi — qaysi fayllarni olish kerak

## Modelga/agentga berish kerak bo'lgan fayllar

| Fayl/papka | Nima | Qachon ishlatiladi |
|---|---|---|
| `chroma_db/` | Tayyor vektor baza (7760 chunk, embedding qilingan, cosine metrika) | **Asosiy variant.** RAG agent to'g'ridan-to'g'ri shu papkani o'qib qidiruv qiladi. Boshqa hech narsa kerak emas. |
| `final_chunks.jsonl` | Yakuniy tozalangan, dedup qilingan chunk'lar (matn + metadata, embeddingsiz) | Agar boshqa vektor bazaga (Qdrant, Pinecone va h.k.) ko'chirmoqchi bo'lsang yoki chunk'larni ko'zdan kechirmoqchi bo'lsang shu faylni ishlat. |

Shu ikkitasi — kerakli hamma narsa. Qolgan hamma narsa **oraliq/ishchi fayllar**, modelga berish shart emas:

| Fayl/papka | Nima uchun kerak emas |
|---|---|
| `raw/` | Xom klonlangan repolar (git tarixi, rasm, kod bilan birga) — chiqib bo'lingan, endi kerak emas |
| `clean/` | Faqat oraliq bosqich (chunk qilishdan oldingi tozalangan .md fayllar) |
| `clean_chunks.jsonl` | Dedup qilinmagan oraliq chunk fayli (final_chunks.jsonl uning tozalangan versiyasi) |
| `scripts/` | Pipeline kodi — bazani qurish uchun, modelga "bilim" sifatida kerak emas |
| `rag_bilim_bazasi_reja.md` | Loyihaning ishchi reja hujjati, bilim bazasiga tegishli emas |

---

## Qo'llanma: qanday ishlatish

### 1-variant: Chroma bazasini to'g'ridan-to'g'ri ishlatish (tavsiya etiladi)

```python
import chromadb
from fastembed import TextEmbedding

client = chromadb.PersistentClient(path="chroma_db")
collection = client.get_collection("shumbola_pentest_kb")

model = TextEmbedding(model_name="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2")
query = "SQL injection qanday tasdiqlanadi?"
q_emb = list(model.embed([query]))[0].tolist()

res = collection.query(query_embeddings=[q_emb], n_results=5)
```

- Embedding modeli **aynan shu bo'lishi shart**: `sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2` — chunki baza shu model bilan vektorlashtirilgan. Boshqa model bilan so'rov yuborsang natijalar noto'g'ri chiqadi.
- `collection.query()` natijasida `documents` (matn), `metadatas` (`source`, `file_path`, `heading`), `distances` (0 ga yaqin = mos) qaytadi.
- Namuna: `scripts/05_query_test.py`

### 2-variant: `final_chunks.jsonl`'ni boshqa tizimga ko'chirish

Har qatorda bitta JSON obyekt:
```json
{"id": "...", "source": "hacktricks", "file_path": "hacktricks/src/.../README.md", "heading": "SQL Injection", "text": "..."}
```
Buni Qdrant/Pinecone/boshqa vektor bazaga yuklashda: har `text`ni o'zing tanlagan embedding modeli bilan vektorla, `id`/`source`/`file_path`/`heading`ni metadata sifatida saqla (manba ko'rsatish uchun kerak).

### Manba tarkibi (7760 chunk)
- hacktricks — 5247
- CheatSheetSeries — 1183
- wstg (OWASP WSTG) — 643
- PayloadsAllTheThings — 393
- Top10 (OWASP Top10) — 208
- API-Security (OWASP) — 86

Barchasi faqat inglizcha, tozalangan (rasm/HTML/navigatsiya shovqinsiz), dublikatlari olib tashlangan.
