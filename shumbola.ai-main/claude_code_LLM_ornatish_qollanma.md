# Claude Code uchun qo'llanma — LLM (Qwen + vLLM) serverga o'rnatish

**Maqsad:** Mavjud Linux serverga shumbola.ai uchun LLM (Qwen) serverini vLLM orqali o'rnatish.
**Eng muhim shart:** Serverda allaqachon boshqa loyihalar, ma'lumotlar bazasi va bir nechta CUDA versiyasi bor. **Ularning birortasiga ham zarar yetmasligi kerak.**

> Ushbu hujjat Claude Code'ga topshiriq sifatida beriladi. Claude Code quyidagi qat'iy qoidalarga rioya qilib, bosqichma-bosqich ishlaydi va har bir tizim darajasidagi o'zgarishdan oldin **rasman ruxsat so'raydi**.

---

## 0. QAT'IY QOIDALAR (Claude Code buzmasligi shart)

Bu qoidalar boshqa loyihalarni himoya qiladi. **Har biri majburiy:**

1. **Drayverga tegma.** NVIDIA drayverini o'chirma, downgrade qilma, qayta o'rnatma. U tizim darajasida umumiy — o'zgartirsang hamma loyihaga ta'sir qiladi. (Zarurat bo'lsa faqat operator qo'lida, alohida.)
2. **Mavjud CUDA'ga tegma.** `/usr/local/cuda*` kataloglarini o'chirma, `/usr/local/cuda` **symlink'ini o'zgartirma**. Mavjud loyihalar unga bog'liq bo'lishi mumkin.
3. **`apt remove` / `apt purge` — TA'QIQLANGAN** cuda/nvidia paketlariga nisbatan.
4. **Global fayllarni o'zgartirma:** `/etc/profile`, `/etc/environment`, tizim `LD_LIBRARY_PATH`, boshqa foydalanuvchilarning `.bashrc`'siga tegma.
5. **Hammasi izolyatsiyada.** O'z CUDA'mizni tizimga emas — **Docker konteyneri** yoki **alohida conda/venv** ichiga o'rnatamiz.
6. **Ma'lumotlar bazasiga tegma.** Serverda DB bor — uning fayllari, portlari, servisiga aralashma.
7. **Diskni to'ldirma.** Model ~30GB joy oladi. Oldin bo'sh joyni tekshir; yetmasa to'xta va so'ra.
8. **Har bir tizim darajasidagi o'zgarishdan oldin — TO'XTA va OPERATORDAN RUXSAT SO'RA.** Faqat o'qish (read-only) buyruqlarni ruxsatsiz bajarish mumkin.
9. **Hech qachon** parol, SSH kalit yoki maxfiy ma'lumotni logga/faylga ochiq yozma.
10. Xatolik yuz bersa — **jim davom etma**. To'xtat, aniq sababni ko'rsat, so'ra.

---

## 1-BOSQICH: Kashfiyot / audit (faqat o'qish, hech narsa o'zgartirilmaydi)

**Maqsad:** serverning holatini tushunish. Bu bosqichda **hech narsa o'rnatilmaydi/o'zgartirilmaydi.** Natijalarni operatorga ko'rsat.

### 1.1. Operatsion tizim va asosiy holat
```bash
# OS versiyasi
cat /etc/os-release
uname -a

# Bo'sh disk joyi (model uchun kamida ~40GB kerak)
df -h

# Xotira
free -h
```

### 1.2. NVIDIA drayveri va GPU holati
```bash
# Drayver versiyasi, GPU'lar, ular qanchalik band
nvidia-smi

# Drayver versiyasi (aniq)
cat /proc/driver/nvidia/version

# Drayver paketi
dpkg -l | grep -i nvidia-driver || true
```
> `nvidia-smi` chiqishidagi "CUDA Version" — bu **drayver qo'llab-quvvatlaydigan MAKSIMAL** CUDA versiyasi, o'rnatilgan versiya emas. Buni chalkashtirma.
> Ikkita RTX 4090 (24GB×2) ko'rinishi kerak. Agar GPU'lar band bo'lsa (boshqa jarayon ishlatayotgan bo'lsa) — operatorga xabar ber.

### 1.3. Mavjud CUDA o'rnatmalari (tegmaymiz, faqat ko'ramiz)
```bash
# Qaysi CUDA versiyalari o'rnatilgan
ls -l /usr/local/ | grep -i cuda || true

# Global symlink qayerga ishora qilyapti (BUNI O'ZGARTIRMAYMIZ)
ls -l /usr/local/cuda || true

# nvcc PATH'da bormi
which nvcc && nvcc --version || echo "nvcc PATH'da yo'q"

# apt orqali o'rnatilgan cuda paketlari
dpkg -l | grep -i cuda || true

# Muhit o'zgaruvchilari
echo "CUDA_HOME=$CUDA_HOME"
echo "LD_LIBRARY_PATH=$LD_LIBRARY_PATH"
```

### 1.4. Mavjud Python muhitlari va boshqa loyihalar
```bash
# conda bormi
which conda && conda env list || echo "conda yo'q"

# python versiyalari
which -a python python3
python3 --version

# Docker bormi (izolyatsiya uchun eng yaxshi yo'l)
which docker && docker --version || echo "docker yo'q"
which nvidia-ctk && nvidia-ctk --version || echo "nvidia-container-toolkit yo'q"

# Ishlab turgan servislar (DB va boshqalarni ko'rish)
docker ps 2>/dev/null || true
systemctl list-units --type=service --state=running | head -50 || true
```

### 1.5. Hisobot
Yuqoridagilarni yig'ib operatorga **qisqa hisobot** ber:
- OS, drayver versiyasi, GPU holati (2×4090 bo'sh yoki band).
- Mavjud CUDA versiyalari va symlink nishoni.
- Docker + nvidia-container-toolkit **bormi yoki yo'qmi** (bu strategiyani belgilaydi).
- Bo'sh disk joyi yetadimi.
- **Hech narsa o'zgartirilmadi.** Keyingi bosqich uchun ruxsat so'ra.

---

## 2-BOSQICH: Strategiya tanlash

Audit natijasiga qarab ikki yo'ldan biri tanlanadi. **Docker yo'li — afzal**, chunki eng to'liq izolyatsiya beradi (o'z CUDA runtime'ini konteyner ichiga oladi, tizimga tegmaydi).

| Holat | Tanlanadigan yo'l |
|-------|-------------------|
| Docker + nvidia-container-toolkit bor (yoki o'rnatsa bo'ladi) | **3A — Docker yo'li** (tavsiya etiladi) |
| Docker yo'q / ishlatib bo'lmaydi | **3B — Conda/venv yo'li** |

**Nega bu boshqa loyihalarga ta'sir qilmaydi:**
- Docker yo'lida CUDA runtime konteyner **ichida** bo'ladi — hostdagi CUDA'ga umuman tegmaydi.
- Conda/venv yo'lida esa PyTorch/vLLM o'z CUDA runtime kutubxonalarini **pip orqali o'z muhitiga** yuklaydi — tizim CUDA'sidan foydalanmaydi.
- Ikkala holatda ham **faqat drayver** umumiy bo'lib qoladi (unga tegmaymiz).

Operatordan qaysi yo'lni tanlashini tasdiqla.

---

## 3A-BOSQICH: Docker yo'li (tavsiya etiladi)

> Har bir `apt install` / tizim o'zgarishidan oldin ruxsat so'ra.

### 3A.1. Docker va NVIDIA Container Toolkit (agar yo'q bo'lsa)
Agar audit'da yo'q chiqsa — bularni o'rnatish tizim darajasidagi o'zgarish, shuning uchun **oldin ruxsat so'ra**. Bor bo'lsa bu qadamni o'tkazib yubor.
- Docker Engine (rasmiy Docker hujjatidagi qadamlar bo'yicha).
- NVIDIA Container Toolkit (rasmiy NVIDIA hujjatidagi qadamlar bo'yicha) — bu konteynerlarga GPU beradi, **hostdagi CUDA'ni o'zgartirmaydi**.

### 3A.2. GPU konteynerda ishlashini tekshirish
```bash
# Konteyner GPU'ni ko'rayaptimi (bu HOST CUDA'sini o'zgartirmaydi)
docker run --rm --gpus all nvidia/cuda:12.4.0-base-ubuntu22.04 nvidia-smi
```
2×4090 ko'rinsa — izolyatsiya to'g'ri ishlayapti.

### 3A.3. Model va ish katalogi
```bash
# Loyiha uchun ALOHIDA katalog (boshqalarga aralashmaydi)
mkdir -p /opt/shumbola/models
# Diskda ~40GB bo'sh joy borligini yana bir bor tekshir
df -h /opt/shumbola
```

### 3A.4. vLLM'ni rasmiy Docker image bilan ishga tushirish
```bash
# ESLATMA: {MODEL_ID}, {QUANT}, {PORT} — Ilovadan (pastda) to'ldiriladi.
# Model kesh katalogi mount qilinadi, shunda qayta yuklab o'tirilmaydi.
docker run -d --name shumbola-llm \
  --gpus all \
  --restart unless-stopped \
  -v /opt/shumbola/models:/root/.cache/huggingface \
  -p {PORT}:8000 \
  vllm/vllm-openai:latest \
  --model {MODEL_ID} \
  --tensor-parallel-size 2 \
  --gpu-memory-utilization 0.90 \
  --max-model-len {CONTEXT_LEN}
```
> `--tensor-parallel-size 2` — modelni **ikkala 4090 o'rtasida** taqsimlaydi (48GB birlashtiriladi).
> `--gpu-memory-utilization` — agar GPU'da boshqa ish bo'lsa pasaytir; hozir bo'sh bo'lsa 0.90 bo'ladi.
> Kvantlash (`{QUANT}`) modeliga bog'liq — Ilovaga qara.

---

## 3B-BOSQICH: Conda/venv yo'li (Docker bo'lmasa)

> Tizim CUDA'siga tegmaydi — PyTorch/vLLM o'z CUDA runtime'ini pip orqali o'z muhitiga oladi.

### 3B.1. Alohida muhit yaratish
```bash
# ALOHIDA conda muhiti (mavjud muhitlarga aralashmaydi)
conda create -n shumbola python=3.11 -y
conda activate shumbola
```
Agar conda yo'q bo'lsa — `python3 -m venv /opt/shumbola/venv` bilan alohida venv yaratiladi (Python versiyasi mos bo'lsa).

### 3B.2. vLLM o'rnatish
```bash
# vLLM o'z torch + CUDA runtime wheel'larini olib keladi (tizim CUDA'si ISHLATILMAYDI)
pip install vllm
```
> Bu muhit ichida bajariladi — global pip'ga tegmaydi.
> **Diqqat:** hech qachon `sudo pip install` qilma va tizim (global) Python'iga o'rnatma.

### 3B.3. Ishga tushirish
```bash
mkdir -p /opt/shumbola/models
export HF_HOME=/opt/shumbola/models

vllm serve {MODEL_ID} \
  --tensor-parallel-size 2 \
  --gpu-memory-utilization 0.90 \
  --max-model-len {CONTEXT_LEN} \
  --port {PORT}
```
Doimiy ishlashi uchun keyinroq buni `systemd` servis qilib qo'yamiz (alohida qadam, operator ruxsati bilan).

---

## 4-BOSQICH: Modelni tekshirish

vLLM OpenAI-mos API beradi. Test so'rov:
```bash
curl http://localhost:{PORT}/v1/models

curl http://localhost:{PORT}/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "{MODEL_ID}",
    "messages": [{"role": "user", "content": "Salom, ishlayapsanmi? Bir jumlada javob ber."}]
  }'
```
Javob kelsa — LLM server ishga tushdi.

Yana tekshir:
```bash
# GPU'lar to'g'ri ishlatilayaptimi (ikkalasida ham xotira band bo'lishi kerak)
nvidia-smi

# Boshqa loyihalar hali joyida (DB, servislar) — buzilmaganini tasdiqla
docker ps 2>/dev/null || true
systemctl list-units --type=service --state=running | head -50 || true
```

---

## 5-BOSQICH: Rollback / tozalash (kerak bo'lsa)

Agar biror narsa noto'g'ri ketsa — **faqat o'zimiz qo'shgan narsani** olib tashlaymiz, boshqasiga tegmaymiz:
```bash
# Docker yo'li
docker stop shumbola-llm && docker rm shumbola-llm
docker rmi vllm/vllm-openai:latest   # ixtiyoriy

# Conda yo'li
conda deactivate
conda env remove -n shumbola

# Model fayllari
rm -rf /opt/shumbola   # DIQQAT: faqat bizning katalog, boshqasiga tegma
```
> Mavjud CUDA, drayver, boshqa loyihalar — **o'zgarmagan** bo'lishi kerak, chunki biz ularga tegmadik.

---

## Ilova: to'ldiriladigan o'zgaruvchilar

O'rnatishdan oldin operator quyidagilarni aniqlaydi:

| O'zgaruvchi | Izoh |
|-------------|------|
| `{MODEL_ID}` | Aniq Qwen model repo IDsi. **Qwen'ning rasmiy HuggingFace sahifasidan tasdiqlash shart** — versiya va aniq nom o'zgargan bo'lishi mumkin. |
| `{QUANT}` | Kvantlash turi. 2×RTX 4090 (Ada, FP8 qo'llab-quvvatlaydi) uchun **FP8** toza tanlov (~27–30GB, vLLM native). Muqobil — **AWQ/GPTQ 4-bit** (kamroq xotira, ko'proq kontekst). GGUF Q8 esa vLLM'da emas, ko'proq `llama.cpp`'da ishlaydi — agar Q8 kerak bo'lsa engine tanlovini qayta ko'rib chiqing. |
| `{CONTEXT_LEN}` | Kontekst uzunligi. 48GB cheklovi sababli boshida ehtiyotkor qiymat (masalan 32k), keyin sozlanadi. |
| `{PORT}` | API porti. **Boshqa servis (DB va h.k.) band qilmagan** portni tanla (`ss -tlnp` bilan tekshir). |

---

## Muhim eslatmalar (operator uchun)

1. **Ulanish maxfiyligi:** Claude Code serverga SSH orqali ulanadi. SSH kalitni **o'z mashinangizda** (`~/.ssh/`, `~/.ssh/config` aliasi) sozlang — **parol/kalitni chatga yozmang**.
2. **Bu qo'llanma versiyalarga bog'liq bilimga tayanadi** (vLLM, Qwen, CUDA tez o'zgaradi). Aniq versiya va model nomini rasmiy manbalardan tasdiqlang: Qwen HuggingFace, vLLM hujjati, NVIDIA hujjati.
3. **Birinchi marta test serverda sinang** (agar mumkin bo'lsa), keyin asosiy serverda.
4. Har bir tizim o'zgarishi **audit log'ga** yozilsin (shumbola falsafasiga mos).

---

*Qo'llanma oxiri — shumbola.ai LLM o'rnatish (Claude Code uchun)*
