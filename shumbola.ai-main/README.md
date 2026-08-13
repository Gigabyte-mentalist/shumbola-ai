# shumbola.ai

Sun'iy intellektga asoslangan, on-premise pentesting yordamchisi. To'liq talablar uchun [shumbola_ai_TZ.md](shumbola_ai_TZ.md) hujjatiga qarang.

## Repo strukturasi

```
services/agent-core/   Go backend — Agent Core (ReAct sikli, HITL, tool'lar, report engine)
services/rag-service/  Python/FastAPI — RAG bilim bazasi (ChromaDB + fastembed)
web-ui/                React + TypeScript operator paneli
docker-compose.yml     Lokal/on-premise deploy uchun servislar
```

## Tarkibiy qismlar va portlar

| Servis | Port | Vazifasi |
|---|---|---|
| `agent-core` (Go) | `8080` | HTTP API — engagement, HITL, task tree, report |
| `web-ui` (Vite) | `5173` | Operator paneli (brauzer) |
| `rag-service` (Python) | `8000` | RAG bilim bazasi (faqat `agent-core` ishlatadi, tashqariga ochish shart emas) |
| LLM (vLLM, GPU server) | `18000` | Qwen3-32B-FP8, OpenAI-mos API |

---

## Ishga tushirish (dev, faqat shu kompyuterda)

### 1. `rag-service` (ixtiyoriy — bo'lmasa `agent-core` RAG'siz ishlaydi)
```powershell
cd services/rag-service
pip install -r requirements.txt
$env:CHROMA_DB_PATH = "C:\Users\Administrator\Desktop\shumbola\chroma_db"
python -m uvicorn app:app --host 0.0.0.0 --port 8000
```

### 2. `agent-core`
```powershell
cd services/agent-core
$env:SEED_OPERATOR_KEY = "dev-operator-key"
$env:LISTEN_ADDR = ":8080"
$env:LLM_BASE_URL = "http://<GPU-server-IP>:18000/v1"   # bo'lmasa StaticPlanner ishlaydi
$env:RAG_SERVICE_URL = "http://localhost:8000"           # ixtiyoriy
go run ./cmd/server
```

Docker o'rnatilgan va ishga tushirilgan bo'lishi shart — aks holda API ishlaydi, lekin haqiqiy tool'lar (nmap, nuclei, ffuf, testssl, smbenum) bajarilmaydi.

### 3. `web-ui`
```powershell
cd web-ui
npm install
npm run dev
```

### 4. Brauzerda ochish
`http://localhost:5173` → server manzili `http://localhost:8080`, kalit `dev-operator-key`.

---

## Lokal tarmoqdagi boshqa kompyuterlar uchun ochish

Standart holatda `agent-core` (Go) allaqachon barcha tarmoq interfeyslarida tinglaydi (`:8080`), lekin Vite dev-server odatda faqat `localhost`da ishlaydi. Buni `web-ui/vite.config.ts`da `server.host: true` orqali hal qilingan — `npm run dev` endi avtomatik LAN'da ham ochiladi va terminalda "Network" manzilini ko'rsatadi:

```
➜  Local:   http://localhost:5173/
➜  Network: http://192.168.X.X:5173/   ← shu manzilni boshqa kompyuterlarga bering
```

**Qadamlar:**

1. Shu kompyuterning lokal tarmoq (LAN) IP manzilini aniqlang:
   ```powershell
   ipconfig | findstr /i "IPv4"
   ```
2. `agent-core`ni yuqoridagidek odatdagi tarzda ishga tushiring (`:8080` allaqachon barcha interfeyslarda tinglaydi).
3. `web-ui`ni `npm run dev` bilan ishga tushiring — terminal LAN manzilini ko'rsatadi.
4. **Windows Firewall**: birinchi marta ishga tushirganda Windows "ruxsat berish/bloklash" oynasini chiqarishi mumkin — **Private network** uchun ruxsat bering. Agar so'ralmasa yoki qo'lda ochish kerak bo'lsa:
   ```powershell
   # Administrator sifatida PowerShell'da:
   New-NetFirewallRule -DisplayName "shumbola agent-core" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
   New-NetFirewallRule -DisplayName "shumbola web-ui" -Direction Inbound -LocalPort 5173 -Protocol TCP -Action Allow
   ```
5. Boshqa kompyuterdagi operator brauzerida **shu kompyuterning LAN IP manzili**ni ochadi, masalan `http://192.168.10.108:5173`, va login formasida server manzili sifatida `http://192.168.10.108:8080` kiritadi (kalit — sizdagi `SEED_OPERATOR_KEY` qiymati).

> **Eslatma (xavfsizlik):** bu faqat ishonchli lokal tarmoq (masalan ofis/uy Wi-Fi) uchun mo'ljallangan dev-rejim. `agent-core`ning CORS sozlamasi hozircha `*` (istalgan origin) — chunki autentifikatsiya cookie emas, `X-API-Key` header orqali, shuning uchun CSRF xavfi yo'q; lekin shunga qaramay tarmoqni faqat ishonchli odamlar bilan bo'lishing.

---

## Test uchun nishonlar (Docker orqali, ixtiyoriy)

Haqiqiy ruxsatsiz tashqi IP/domenni skanerlash noqonuniy bo'lishi mumkin — shuning uchun lokal, ataylab zaif qilib qurilgan konteynerlar tavsiya etiladi:

```powershell
# Zaif web-ilova (DVWA) — nuclei/ffuf/testssl uchun
docker run -d --name shumbola-dvwa --restart unless-stopped -p 8081:80 vulnerables/web-dvwa

# SMB xizmati (Samba) — smbenum uchun
docker run -d --name shumbola-samba --restart unless-stopped -p 1445:445 dperson/samba:latest -s "public;/tmp;yes;no;yes;all" -u "testuser;testpass"
```

Nishon sifatida konteyner IP manzilini ishlating (host tarmog'idagi `bridge` tarmoq IP'i):
```powershell
docker inspect -f "{{.NetworkSettings.Networks.bridge.IPAddress}}" shumbola-dvwa
docker inspect -f "{{.NetworkSettings.Networks.bridge.IPAddress}}" shumbola-samba
```

---

## Mavjud tool'lar

| Tool | Vazifasi |
|---|---|
| `nmap` | Port/xizmat skanerlash |
| `smbenum` | SMB (139/445-port) enumeratsiyasi — OS, protokol, shares (faqat o'qish) |
| `nuclei` | CVE/zaiflik shablonlari bo'yicha tekshiruv |
| `ffuf` | Fayl/katalog fuzzing |
| `testssl` | TLS/SSL konfiguratsiyasi tekshiruvi |

Har biri HITL (inson tasdig'i) orqali o'tadi, faqat berilgan scope ichidagi nishonlarga qarshi ishlaydi.

---

## Docker Compose (agent-core + web-ui)
```powershell
docker compose up --build agent-core web-ui
```

vLLM xizmati `gpu` compose profili ostida (GPU'li serverlarda ishga tushiriladi):
```powershell
docker compose --profile gpu up --build
```

## Holat

- Bosqich 0-3: tayyor (poydevor, agent yadrosi, tool va validatsiya, report va til)
- Bosqich 4: RAG bilim bazasi ✅, Web UI ✅, ichki test muhitida sinov ✅ — **LoRA fine-tune hali boshlanmagan**
- To'liq roadmap uchun [shumbola_ai_TZ.md](shumbola_ai_TZ.md) 10-bo'limiga qarang.
