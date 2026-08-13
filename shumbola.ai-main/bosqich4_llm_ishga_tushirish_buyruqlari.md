# Bosqich 4 — vLLM ishga tushirish buyruqlari (operator o'zi bajaradi)

Serverga o'zingiz SSH orqali kiring (`ssh shumbola-server`, passphrase bilan), so'ng quyidagi buyruqlarni **ketma-ket** bajaring. Har birida `sudo` parolingiz so'raladi.

## 1. Ish katalogini yaratish

```bash
sudo mkdir -p /opt/shumbola/models
sudo chown shumbola:shumbola /opt/shumbola/models
df -h /opt/shumbola
```

## 2. vLLM image'ni tortib olish

```bash
sudo docker pull vllm/vllm-openai:latest
```

## 3. vLLM konteynerini ishga tushirish

```bash
sudo docker run -d --name shumbola-llm \
  --gpus all \
  --restart unless-stopped \
  -v /opt/shumbola/models:/root/.cache/huggingface \
  -p 8000:8000 \
  vllm/vllm-openai:latest \
  --model Qwen/Qwen3.6-27B-FP8 \
  --tensor-parallel-size 2 \
  --gpu-memory-utilization 0.90 \
  --max-model-len 32768
```

Bu buyruq darhol tugaydi (konteyner fon rejimida ishga tushadi), lekin model haqiqatan tayyor bo'lishi uchun (~27GB checkpoint yuklab olinishi + GPU'ga joylashishi) bir necha daqiqa ketishi mumkin.

## 4. Loglarni kuzatish (ixtiyoriy, real vaqtda)

```bash
sudo docker logs -f shumbola-llm
```

`INFO: ... Uvicorn running on http://0.0.0.0:8000` yoki shunga o'xshash "ready"/"application startup complete" xabari chiqsa — model tayyor. `Ctrl+C` bilan log kuzatuvidan chiqishingiz mumkin (konteyner ishlashda davom etadi).

---

Bajarib bo'lgach menga xabar bering — men (sudo'siz, read-only tekshiruvlar bilan) modelning API orqali javob berishini va boshqa GPU/konteynerlarga ta'sir qilmaganini tekshiraman.
