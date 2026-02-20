FROM python:3.11-slim

WORKDIR /app

# Install system dependencies for OpenCV and image processing
RUN apt-get update && apt-get install -y \
    libgl1 \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender-dev \
    libgomp1 \
    wget \
    curl \
    g++ \
    libopenblas-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Pre-download the face model (buffalo_l is ~300MB)
# This caches the model in the image so container starts faster
RUN python -c "from insightface.app import FaceAnalysis; \
    app = FaceAnalysis(name='buffalo_l', providers=['CPUExecutionProvider']); \
    app.prepare(ctx_id=-1, det_size=(640, 640)); \
    print('Model downloaded successfully')" || echo "Model download skipped (will download on first use)"

# Copy application code
COPY main.py .

# Environment variables with defaults
ENV PORT=8000
ENV MATCH_THRESHOLD=0.45
ENV QUALITY_THRESHOLD=0.3
ENV DETECTION_SIZE=640
ENV USE_GPU=false
ENV REDIS_URL=""

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD python -c "import requests; requests.get('http://localhost:8000/health')" || exit 1

# Run the service
CMD ["python", "main.py"]
