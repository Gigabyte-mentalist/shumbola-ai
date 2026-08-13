# Yuklab olish tugaganini qanday bilish mumkin

Serverga kiring (`ssh shumbola-server`), so'ng:

## 1. Tezkor holat (bitta buyruq)

```bash
sudo docker ps -a --filter name=shumbola-model-download --format '{{.Status}}'
```

- **`Up X minutes`** — hali yuklanmoqda (jarayon davom etyapti).
- **`Exited (0) ...`** — muvaffaqiyatli tugadi.
- **`Exited (1) ...`** yoki boshqa kod — barcha 20 qayta urinish tugab, muvaffaqiyatsiz yakunlandi (menga xabar bering).

## 2. Progress (necha GB yuklangan)

```bash
du -sh /opt/shumbola/models/hub/models--Qwen--Qwen3-32B-FP8/blobs
```

Bu son **~34 GB**ga yetganda (masalan `33G`, `34G`) — deyarli tugagan bo'ladi.

## 3. Aniq "tugadi" belgisi (loglarda)

```bash
sudo docker logs shumbola-model-download | grep DONE
```

Agar `DONE: /root/.cache/huggingface/hub/models--Qwen--Qwen3-32B-FP8/snapshots/...` chiqsa — **100% tugagan**, keyingi bosqichga (vLLM'ni ishga tushirish) o'tsa bo'ladi.

## 4. Real vaqtda kuzatish (ixtiyoriy)

```bash
sudo docker logs -f shumbola-model-download
```

`Ctrl+C` bilan chiqish mumkin (yuklab olish davom etaveradi, faqat kuzatuvdan chiqasiz).

---

Menga shunchaki "tugadimi tekshir" deb yozsangiz ham bo'ladi — men xuddi shu buyruqlarni ishlataman.
