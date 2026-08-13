FROM python:3.11-slim
WORKDIR /app

# fastembed's ONNX runtime and chromadb both need a C++ runtime; slim
# already has it, but keep this minimal and avoid adding build tooling
# we don't need (no compiled deps in requirements.txt).
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

RUN adduser --disabled-password --gecos "" shumbola
USER shumbola

ENV CHROMA_DB_PATH=/data/chroma_db
EXPOSE 8000
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
