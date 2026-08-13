# Claude Code uchun brief — shumbola-server

Bu qisqa hujjat Claude Code'ga: (1) serverga qanday ulanish, (2) qaysi vazifani bajarishni ko'rsatadi. Batafsil texnik qoidalar alohida faylда (`claude_code_LLM_ornatish_qollanma.md`) — Claude Code o'sha qo'llanma asosида ish ko'radi.

---

## 1. Serverga ulanish

Server SSH orqали sozlangan. Ulanish uchun:

```bash
ssh shumbola-server
```

- Bu nom (`shumbola-server`) allaqachon `~/.ssh/config` faylида sozlangan (HostName `192.168.10.201`, User `shumbola`, kalit `~/.ssh/shumbola_key`).
- Ulanганда SSH **kalit passphrase**'ini so'raydi — bu operator (inson) tomonidан kiritiladi.
- Server ichида foydalanuvchi: **`shumbola`** (root emas). Tizim darajасидаги ishlar uchun `sudo` ishlatiladi va u **parol so'raydi** — operator kiritadi.
- Server: **CentOS 7**, 2×RTX 4090 (48GB), Docker + NVIDIA Container Toolkit o'rnatilган.

> Operator uchun eslatma: passphrase/parol har safar qo'lда kiritiladi (avtomatик saqlanmagan). Claude Code ulanишни boshlаганда, operator shu ikki narsani kiritishga tayyor tursin.

---

## 2. Asosiy vazifa

**shumbola.ai loyihasi uchun LLM serverini (Qwen + vLLM) shu serverга Docker konteynerида o'rnatish va ishga tushirish.**

Batafsil bosqichlar, qat'iy xavfsizlik qoidалари va buyruqlar `claude_code_LLM_ornatish_qollanma.md` faylида. Claude Code o'sha qo'llanмани ket-ket bajaradi.

### Eng muhim qoidalar (buzilmasin)
1. **Boshqa loyihalarга zarar yetkazма.** Serverда boshqa ishlар (masalан YOLO/Ultralytics o'qitiши) va ma'lumotlар бор.
2. **Tizim CUDA'siга (`/usr/local/cuda-12.4`), drayverга (`550.54.14`), symlink'га tegма.** Hammаси Docker konteynerида izolyatsiyalanган.
3. **Har bir tизим darajасидаги o'zgаришдан oldin (`sudo`, paket o'rnатиш) — TO'XTA va operatordан RUXSAT SO'RA.**
4. Faqat o'qиш (read-only) buyruqларни ruxsatsiz bajарса bo'ladi.
5. **GPU bo'lишилган** — konteyner ishлаганда ikkала GPU'ни egallаydi; operator bilan navbатни kelишиб ol.

### Tez xulоsа (audit allaqачон qilинган)
- OS: CentOS 7 | Drayver: 550.54.14 (CUDA 12.4'gacha) | GPU: 2×RTX 4090 = 48GB
- Docker 24.0.9 + nvidia-ctk 1.14.6 **bор** → **Docker yo'li** ishlatilади.
- Disk: 1.9TB+ bo'sh → joy yetарли.
- Tanlанган model: **`Qwen/Qwen3.6-27B-FP8`**, `--tensor-parallel-size 2`, `--max-model-len 32768`.

---

## 3. Kutилаётган natija

- vLLM konteyner ishga tushади, OpenAI-mos API **8000-portда** javob beradi.
- Test: `curl http://localhost:8000/v1/chat/completions` → model javob berса, tayyor.
- Boshqa loyihалар (CUDA, YOLO, DB) **buzилмаган** bo'lиши kerak.

---

## 4. Loyiha konteksti (TZ)

Bu LLM — shumbola.ai (inson-nazoратидаги avtomatlashtирилган pentest yordamchиси) uchun "miya". To'liq loyиha talablari, arxitektura va bosqичlар `shumbola_ai_TZ.md` faylида. Hozirги vazifa — o'sha TZ'ning **infratuzилма bosqичи** (LLM'ни ishga tushириш). Model ishga tushгач, keyинги bosqич — agent qatlами (ReAct sikli, tool orkestратsiyа) TZ asosида quriлади.
