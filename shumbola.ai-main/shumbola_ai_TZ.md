# shumbola.ai — Texnik Topshiriq (TZ)

**Loyiha:** shumbola.ai — sun'iy intellektga asoslangan pentesting yordamchisi (AI-powered penetration testing assistant)
**Hujjat turi:** Texnik Topshiriq (v1.0, ishlab chiqishga asos)
**Sana:** 2026
**Maxfiylik:** Ichki foydalanish uchun

---

## 0. Hujjat haqida

Ushbu hujjat shumbola.ai mahsulotining birinchi versiyasini (v1 / MVP) ishlab chiqish uchun texnik asos hisoblanadi. U maqsad, doira, funksional va nofunksional talablar, arxitektura, xavfsizlik talablari, jamoa ish taqsimoti, bosqichlar va qabul qilish mezonlarini belgilaydi. TZ tirik hujjat — bosqichlar davomida aniqlashtirilib boriladi.

### 0.1. Atamalar va qisqartmalar

| Atama | Izoh |
|-------|------|
| **LLM** | Katta til modeli (miya). Ushbu loyihada Qwen 3.6 27B. |
| **Agent** | LLM ustiga qurilgan tizim: fikrlash sikli, tool chaqirish, holat, boshqaruv mantiqi. |
| **ReAct** | Reasoning + Acting — "o'yla → harakat qil → natijani ko'r → yana o'yla" sikli. |
| **Attack surface** | Hujum sathi — test qilinadigan tizim qismlari (web, tarmoq va h.k.). |
| **Recon** | Razvedka — nishon haqida ma'lumot to'plash bosqichi. |
| **PoC** | Proof of Concept — zaiflikni buzmasdan isbotlash. |
| **False positive** | Yolg'on ijobiy — mavjud bo'lmagan zaiflikni xato aniqlash. |
| **HITL** | Human-in-the-loop — inson nazorati/tasdig'i nuqtasi. |
| **Task Tree (PTT)** | Pentesting Task Tree — agent holatini saqlaydigan vazifalar daraxti. |
| **RAG** | Retrieval-Augmented Generation — bilim bazasidan kontekst yuklab modelga berish. |
| **LoRA** | Modelni kam resursda moslashtirish (fine-tuning) usuli. |
| **vLLM** | LLM'ni yuqori unumdorlik bilan xizmat qiluvchi server dvigateli. |
| **Kill switch** | Favqulodda to'xtatish tugmasi. |
| **Scope** | Test doirasi — ruxsat berilgan nishonlar ro'yxati. |

---

## 1. Loyiha maqsadi va vazifalari

### 1.1. Muammo
Pentesting qimmat, sekin va inson resursiga bog'liq. O'zbekiston va Markaziy Osiyoda malakali pentesterlar yetishmaydi, xalqaro AI-yechimlar esa bulutli bo'lgani uchun ma'lumotni chegaradan chiqara olmaydigan banklar va davlat organlariga qonunan mos kelmaydi.

### 1.2. Yechim
shumbola.ai — mijozning **o'z serverida (on-premise)** ishlaydigan, sun'iy intellektga asoslangan pentesting yordamchisi. U inson mutaxassis nazorati ostida razvedka, zaiflik tekshiruvi va tasdiqlashni avtomatlashtiradi, hamda xalqaro standartlarga mos report generatsiya qiladi.

### 1.3. Asosiy ustunlik (moat)
- **Lokal / on-premise** — ma'lumot mijoz binosidan chiqmaydi.
- **Akkreditatsiya va ishonch** — davlat ahamiyatiga ega tashkilot ortida.
- **Mahalliy moslik** — o'zbek/rus tili, mahalliy compliance talablari.
- **Inson nazorati** — to'liq avtonom emas; xato xavfi past.

### 1.4. Biznes maqsad
Malakali pentester ishini tezlashtirish va inson xatolarini kamaytirish; bir martalik yoki doimiy xizmat shaklida shartnoma asosida taqdim etish.

### 1.5. Muvaffaqiyat mezoni (v1 uchun)
- Standart web-ilova va tarmoq testini inson operatordan **kamida 3–5 barobar tez** yakunlash.
- Report'dagi false positive darajasi **belgilangan chegaradan past** (validatsiya qatlami hisobiga).
- Bitta mutaxassis nazorati ostida to'liq engagement'ni oxiriga yetkazish.

---

## 2. Mahsulot doirasi (Scope)

### 2.1. v1 (MVP) doirasiga KIRADI
- **Hujum sathi:** web-ilovalar (external) + tarmoq infratuzilmasi (external/internal).
- **Chuqurlik:** zaiflik topish + **buzmasdan tasdiqlash (non-destructive PoC)**.
- **Deploy:** mijoz serveriga o'rnatish (on-premise).
- **Til:** o'zbek (lotin), o'zbek (kirill), rus; report standartlari uchun ingliz atamalari.
- **Report:** avtomatik, standartlarga bog'langan (5-bo'lim).
- **Nazorat:** inson-nazoratli (HITL) rejim.

### 2.2. v1 doirasiga KIRMAYDI (keyingi versiyalar)
- To'liq avtonom rejim.
- Buzuvchi (destructive) exploitatsiya.
- Active Directory, bulut (cloud) infratuzilmasi, wireless, mobil ilova, ijtimoiy injiniring.
- SaaS / markazlashgan bulutli xizmat (bu — v2 rejasi, o'z kuchli serverimizda).
- PCI DSS / ISO 27001 to'liq audit reportlari (banklar bosqichida qo'shiladi).

### 2.3. Deploy modeli
- **v1:** har bir mijozning serveriga alohida o'rnatiladi. Tizim to'liq izolyatsiyalangan, internetsiz (offline) ishlay oladi. Minimal apparat spetsifikatsiyasi standartlashtiriladi (7-bo'limga qarang).
- **v2 (kelajak):** o'z kuchli serverimizda kuchli modellar bilan, xavfsiz tarzda markazlashgan xizmat.

---

## 3. Foydalanuvchilar va rollar

| Rol | Vakolat |
|-----|---------|
| **Operator (pentester mutaxassis)** | Security key bilan kiradi. Scope belgilaydi, agentga topshiriq beradi, harakatlarni nazorat qiladi, HITL gate'larda tasdiqlaydi/rad etadi, istalgan payt to'xtata oladi. |
| **Administrator** | Tizimni o'rnatadi/sozlaydi, foydalanuvchilarni boshqaradi, apparatni nazorat qiladi. |
| **Auditor (ixtiyoriy)** | Audit loglarni ko'radi (o'zgartirmasdan), muvofiqlikni tekshiradi. |

> v1'da barcha aktiv harakatlar faqat autentifikatsiyadan o'tgan operator tasdig'i bilan amalga oshiriladi.

---

## 4. Funksional talablar

### 4.1. Pentest jarayoni bosqichlari
Agent quyidagi bosqichlarni ketma-ket va moslashuvchan tarzda bajaradi:

1. **Scope belgilash** — operator ruxsat berilgan nishonlar (IP, domen, port diapazoni) ro'yxatini kiritadi. Bu allowlist tizim darajasida majburlanadi.
2. **Razvedka (recon)** — passiv va aktiv ma'lumot to'plash (portlar, xizmatlar, texnologiyalar).
3. **Zaiflik aniqlash (scanning)** — tool'lar orqali potensial zaifliklarni topish.
4. **Tasdiqlash (validation / PoC)** — topilgan zaiflikni buzmasdan isbotlash, false positive'larni filtrlash.
5. **Report** — natijalarni standartlarga bog'lab, tanlangan tillarda hujjatlashtirish.

### 4.2. Agent qatlami (Agent Core)
- **ReAct sikli:** har qadamda model holatni tahlil qiladi → keyingi harakatni tanlaydi → tool'ni chaqiradi → natijani o'qiydi → task tree'ni yangilaydi.
- **Task Tree (PTT):** engagement holati strukturali daraxtda saqlanadi (nishon → topilma → keyingi qadamlar). Kontekst cheklovi (48GB) sababli holat xulosalab (summarize) boriladi.
- **Tiklanish mantiqi:** reja barbod bo'lsa (tool xato berdi, natija bo'sh), agent muqobil yo'lni sinaydi yoki operatordan so'raydi.
- **To'xtash sharti:** vazifa yakunlanganda, resurs chegarasiga yetganda yoki operator to'xtatganda.

### 4.3. Tool integratsiyasi qatlami (Tool Layer)
- Agent standart, ochiq pentesting tool'larini boshqaradi (**model o'zi exploit yozmaydi — tool'lar topadi, model orkestrlaydi**).
- **Web:** masalan `nuclei`, `nikto`, `wpscan`, `ffuf`/`gobuster` (katalog/fayl topish), `testssl.sh` (TLS), `sqlmap` (faqat xavfsiz/read-only bayroqlar bilan).
- **Tarmoq:** masalan `nmap`, `masscan` (aniqlash), xizmat versiyalarini aniqlash.
- Har bir tool **wrapper** orqali chaqiriladi: kirish parametrlari tekshiriladi, chiqishi strukturaga o'giriladi (parsing), xavfli bayroqlar bloklanadi.
- **Intruziv/buzuvchi bo'lishi mumkin bo'lgan har bir tool chaqiruvi HITL gate'dan o'tadi** (4.4).
- Yangi tool qo'shish oson bo'lishi uchun modulli interfeys (pluggable). Runtime'da tool qo'shish uchun MCP yondashuvidan foydalanish mumkin.

### 4.4. Inson nazorati nuqtalari (HITL Gates)
Operator tasdig'i majburiy bo'lgan holatlar:
- Scope birinchi marta belgilanganda.
- Har qanday **aktiv/intruziv** harakat oldidan.
- Validator "xavfli bo'lishi mumkin" deb belgilagan har bir harakat oldidan.
- Buzuvchi ta'sir ehtimoli bor tool bayroqlari uchun.

Operator xatoni sezsa — **darhol to'xtata** oladi, agent shu ondayoq bo'ysunadi (kill switch, 6-bo'lim).

### 4.5. Tasdiqlash / validatsiya qatlami (Validation)
- Har bir topilma **proof-of-exploit** mezoniga tekshiriladi: "isbotlanmagan bo'lsa — reportga chiqmaydi".
- Buzmasdan takrorlanadigan dalil to'planadi (so'rov/javob, ekran, chiqish jurnali).
- Ishonch darajasi (confidence) belgilanadi; past ishonchli topilmalar operator ko'rigiga yuboriladi.

### 4.6. Report generatsiya moduli
- **Formatlar:** PDF (asosiy), va tahrirlanuvchi format (masalan DOCX/HTML).
- **Tillar:** o'zbek (lotin), o'zbek (kirill), rus; standart atamalar uchun ingliz.
- **Tarkib:** boshqaruv xulosasi (executive summary), topilmalar (jiddiylik bo'yicha), texnik tafsilotlar, dalillar, tuzatish tavsiyalari (remediation), standartlarga xarita.
- **Standartlar (5-bo'lim):** OWASP Top 10, MITRE ATT&CK, CVSS baholash + mahalliy talablar.
- Har topilma → jiddiylik (severity), CVSS ball, ta'sir, dalil, tuzatish tavsiyasi.

### 4.7. Interfeys (UI)
- **Web-asosli boshqaruv paneli** (mijoz serverida lokal ishlaydi).
- Operator: engagement yaratish, scope kiritish, agent harakatlarini real vaqtda kuzatish, HITL tasdiqlash, to'xtatish, report ko'rish/eksport.
- Real vaqtli jurnal (live log) va task tree vizualizatsiyasi.
- Tillar: o'zbek (lotin/kirill), rus.

---

## 5. Report standartlari

v1 report quyidagilarga bog'lanadi:
- **OWASP Top 10** — web zaifliklari tasnifi.
- **MITRE ATT&CK** — texnika/taktika xaritasi.
- **CVSS (v3.1/v4)** — jiddiylikni sonli baholash.
- **Mahalliy talablar** — O'zbekiston DXX / O'zDSt kiberxavfsizlik talablariga muvofiqlik bo'limlari.

> Banklar bosqichida (keyingi versiya) PCI DSS va ISO 27001 xaritalari qo'shiladi. TZ shu kengaytma uchun modulli report shablonini nazarda tutadi.

---

## 6. Nofunksional talablar

### 6.1. Xavfsizlik (KRITIK — majburiy)
- **Scope enforcement:** agent scope allowlist'dan tashqari birorta nishonga **jismonan tegilmasin** (tizim darajasida bloklangan, faqat model "ishonchiga" tayanmaydi).
- **To'liq audit log:** har bir harakat, tool chaqiruvi, qaror, HITL tasdig'i vaqt tamg'asi bilan yozib boriladi. Log o'zgartirilmaydigan (append-only) bo'lishi kerak.
- **Kill switch:** operator bitta harakat bilan barcha faoliyatni darhol to'xtata oladi.
- **Sandbox/izolyatsiya:** tool'lar izolyatsiyalangan muhitda (masalan konteyner) ishlaydi.
- **Autentifikatsiya:** security key orqali kirish; rollarga asoslangan ruxsat (RBAC).
- **Ma'lumot bilan ishlash:** barcha ma'lumot lokal saqlanadi, tashqariga yuborilmaydi; maxfiy ma'lumot shifrlangan holda saqlanadi.
- **Non-destructive kafolati:** production muhitida buzuvchi harakatlar avtomatik amalga oshirilmaydi.

### 6.2. Ishlash unumdorligi (Performance)
- Model 48GB (2×RTX 4090 24GB) apparatда vLLM orqali xizmat qilinadi (tensor parallelism).
- Kontekst byudjeti cheklangani sababli holat xulosalanadi; uzun sessiyalar uchun task tree ixchamlashtiriladi.
- Agentic sikl uchun maqbul token tezligi ta'minlanishi kerak (interaktiv operator kutmasin).

### 6.3. Ishonchlilik va kuzatuvchanlik (Reliability & Observability)
- Har bir engagement qayta tiklanadigan (resume) bo'lishi kerak (sessiya saqlanadi).
- Tizim jarayon holatini, xatolarni va resurs ishlatilishini kuzatib boradi.
- Xatolik yuz berganda agent to'xtaydi va operatorga aniq sabab ko'rsatadi (jim yiqilmaydi).

### 6.4. O'rnatiluvchanlik (Deployability)
- Offline o'rnatish imkoniyati (internetsiz muhit uchun).
- Konteynerlashtirilgan (Docker) yetkazib berish.
- Standart minimal apparat spetsifikatsiyasi hujjatlashtiriladi.

### 6.5. Til
- Interfeys va report: o'zbek (lotin), o'zbek (kirill), rus. Model ko'p tilli (Qwen — rus tilida yaxshi), report shablonlari uch tilda tayyorlanadi.

---

## 7. Texnik arxitektura

### 7.1. Komponentlar

```
┌─────────────────────────────────────────────────────────┐
│                    Web UI (operator paneli)              │
│         engagement / scope / kuzatuv / HITL / report     │
└───────────────────────────┬─────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────┐
│                      AGENT CORE                          │
│  ReAct sikli │ Task Tree (PTT) │ HITL gate mantiqi       │
│  qaror mantiqi │ holat xotirasi │ to'xtash sharti        │
└───┬───────────────┬──────────────┬──────────────┬────────┘
    │               │              │              │
┌───▼─────┐   ┌─────▼─────┐   ┌────▼──────┐  ┌────▼───────┐
│  LLM     │   │  Tool     │   │ Validation│  │  Report    │
│ serving  │   │  Layer    │   │  qatlami  │  │  engine    │
│ (vLLM +  │   │ (nmap,    │   │ (PoC,     │  │ (PDF/DOCX, │
│ Qwen 27B)│   │ nuclei..) │   │ conf.)    │  │ standartlar)│
└───┬──────┘   └───────────┘   └───────────┘  └────────────┘
    │
┌───▼──────────────┐
│  RAG / bilim      │
│  bazasi (security │
│  playbook, CVE)   │
└──────────────────┘

        ┌─────────────────────────────────┐
        │  Audit log (append-only) │ Kill  │
        │  switch │ Sandbox │ RBAC/Auth     │
        │  (barcha komponentlar ustidan)    │
        └─────────────────────────────────┘
```

### 7.2. Model qatlami
- **Base model:** Qwen 3.6 27B (Q8 kvantlash, ~30GB).
- **Serving:** vLLM, 2×RTX 4090 (24GB×2) da tensor parallelism (`tensor-parallel-size=2`).
- **Muqobil sinov:** agentic uchun kontekst yetsa Laguna XS 2.1 (33B/3B MoE) sinovdan o'tkaziladi.
- **Moslashtirish:** curated pentest ma'lumotiga LoRA fine-tune (kamida ~500+ sifatli misol) + RAG bilim bazasi (playbook, CVE, mahalliy standartlar).

### 7.3. Ma'lumot oqimi
Operator scope kiritadi → Agent Core razvedka rejasini tuzadi → Tool Layer'ni chaqiradi → natija Validation'dan o'tadi → topilmalar Task Tree'ga yoziladi → intruziv qadamlar HITL gate'dan o'tadi → yakunda Report Engine hujjat yaratadi. Har bir qadam Audit log'ga yoziladi.

---

## 8. Modelni tayyorlash (Model & Knowledge)

- **Bilim bazasi (RAG):** pentest metodologiyasi, tool foydalanish qoidalari, CVE ma'lumotlari, OWASP/MITRE/CVSS ma'lumotnomalari, mahalliy standartlar. Cyber mutaxassis tayyorlaydi.
- **Fine-tune (LoRA):** mahalliy stsenariylar, o'zbek/rus tilidagi report uslubi, qaror mantiqi bo'yicha misollar. 48GB apparat LoRA uchun yetarli.
- **Skill/prompt qatlami:** har bosqich uchun aniq ko'rsatmalar (SKILL.md yondashuvi) — lekin bu prompt kontekst ekanligi, "o'rgatish" emasligi yodda tutiladi; asosiy moslik RAG + LoRA orqali.
- **Diqqat:** tashqi manbadan (GitHub) olingan har bir skill/komponent ishlatishdan oldin xavfsizlik tekshiruvidan o'tkaziladi (zararli skillar keng tarqalgan).

---

## 9. Jamoa va ish taqsimoti

| A'zo | Mas'uliyat |
|------|-----------|
| **AI Engineer 1 — Agent qatlami** | ReAct sikli, tool wrapper'lar, Task Tree/holat, HITL gate mantiqi, vLLM serving, kill switch integratsiyasi. |
| **AI Engineer 2 — Model & Report** | Model tanlash/kvantlash, LoRA pipeline, RAG bilim bazasi, prompt/skill muhandisligi, report generatsiya moduli (standartlar, tillar). |
| **Cyber mutaxassis — Metodologiya & Validatsiya** | Pentest playbook'lari, RAG/fine-tune uchun xavfsizlik ma'lumotini tayyorlash, proof-of-exploit mezonlari, scope/xavfsizlik qoidalari, natijalarni QA tekshiruvi. |

> **Xavf:** cyber mutaxassis — yagona zaif nuqta (single point of failure). O'sish bosqichida ikkinchi xavfsizlik mutaxassisi kerak bo'ladi. Bilim bazasi va playbook'lar hujjatlashtirilishi shart, toki bilim bitta odamda qamalib qolmasin.

---

## 10. Ishlab chiqish bosqichlari (Roadmap / Milestones)

### Bosqich 0 — Poydevor (Setup)
- Apparat sozlash (2×RTX 4090, vLLM, Qwen 27B ishga tushirish).
- Repozitoriy, konteyner, CI, loyihaviy skelet.
- Xavfsizlik asoslari: audit log, RBAC, sandbox skeleti.
- **Natija:** model lokal serverda javob beradi; asosiy infratuzilma tayyor.

### Bosqich 1 — Agent yadrosi (Core loop)
- ReAct sikli, Task Tree, oddiy tool wrapper (avval `nmap`).
- HITL gate mexanizmi + kill switch.
- **Natija:** agent bitta oddiy tool bilan nazorat ostida razvedka qila oladi.

### Bosqich 2 — Tool va validatsiya
- Web va tarmoq tool'lari to'plamini qo'shish (wrapper + parsing).
- Validation qatlami + proof-of-exploit.
- Scope enforcement (allowlist majburlash).
- **Natija:** agent zaiflik topib, buzmasdan tasdiqlaydi; false positive filtrlanadi.

### Bosqich 3 — Report va til
- Report engine (PDF/DOCX), standartlarga xarita.
- Uch tilli shablonlar (o'zbek lotin/kirill, rus).
- **Natija:** to'liq engagement standartga mos report bilan yakunlanadi.

### Bosqich 4 — Moslashtirish va sinov
- RAG bilim bazasi + LoRA fine-tune.
- Ichki test muhitida (masalan mashg'ulot maqsadidagi zaif tizimlar) sinov.
- UI'ni sayqallash, kuzatuvchanlik.
- **Natija:** MVP tayyor, pilotга tayyor.

### Bosqich 5 — Pilot
- Nazorat qilinadigan mijoz muhitida (test/staging) pilot.
- Fikr-mulohaza asosida takomillashtirish.

---

## 11. MVP qabul qilish mezonlari (Acceptance Criteria)

MVP quyidagilarni bajara olganda tayyor hisoblanadi:
1. Operator security key bilan kiradi, engagement yaratadi va scope belgilaydi.
2. Agent scope ichida razvedka va zaiflik tekshiruvini o'tkazadi; scope'dan chiqmaydi (texnik tasdiqlangan).
3. Har bir intruziv harakat HITL gate'dan o'tadi; operator to'xtata oladi (kill switch ishlaydi).
4. Topilmalar validatsiyadan o'tadi; asossizlari filtrlanadi (proof-of-exploit).
5. Standartlarga bog'langan, tanlangan tildagi report avtomatik generatsiya qilinadi.
6. Barcha harakatlar audit log'ga to'liq yoziladi.
7. Tizim mijoz serveriga offline o'rnatila oladi.

---

## 12. Xatarlar va yumshatish (Risks & Mitigation)

| Xavf | Yumshatish |
|------|-----------|
| Lokal model sifati frontier'dan past → murakkab masalada qoqiladi | Scope'ni torlashtirish; model = orkestrator, tool = topuvchi; HITL; RAG+LoRA. |
| False positive → ishonch yo'qoladi | Proof-of-exploit majburiy; ishonch darajasi; operator QA. |
| Agent scope'dan chiqishi → huquqiy/mas'uliyat xavfi | Tizim darajasida allowlist; sandbox; audit; kill switch. |
| Production tizimni buzish | v1'da faqat non-destructive; buzuvchi harakatlar bloklangan; iloji bo'lsa staging. |
| Cyber mutaxassis — yagona zaif nuqta | Bilim bazasini hujjatlashtirish; keyin ikkinchi mutaxassis. |
| 48GB kontekst cheklovi | Holatni xulosalash; ixcham task tree; kerak bo'lsa apparatni kengaytirish. |
| Tashqi skill/komponent zararli bo'lishi | O'rnatishdan oldin har birini tekshirish; faqat ishonchli manbalar. |
| Huquqiy/litsenziya talablari | DXX akkreditatsiyasi tashkilot orqali oldindan hal qilinadi; yuridik maslahat. |

---

## 13. Kelajakdagi rejalar (v2+)

- O'z kuchli serverimizda kuchli modellar bilan markazlashgan xavfsiz xizmat.
- Buzuvchi exploitatsiya (faqat inson tasdig'i bilan).
- Active Directory, bulut, wireless, mobil sathi.
- To'liq PCI DSS / ISO 27001 audit reportlari.
- Global bozor uchun yangi yechimlar.

---

## 14. Hal qilish kutilayotgan (keyingi aniqlashtirish uchun)

Ushbu nuqtalar ishlab chiqish davomida aniqlanadi (v1'ni bloklamaydi):
- Aniq tool ro'yxatining yakuniy tarkibi va har biriga xavfsiz bayroqlar to'plami.
- Fine-tune uchun ma'lumot to'plash jarayoni va hajmi.
- Report shablonlarining aniq dizayni va tashkilot brendingi.
- Minimal apparat spetsifikatsiyasining aniq raqamlari (mijoz uchun standart).
- Pilot uchun test muhitini tanlash.

---

*Hujjat oxiri — shumbola.ai TZ v1.0*
