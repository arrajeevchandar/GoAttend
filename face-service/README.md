# Face Recognition Microservice

A FastAPI-based face recognition service using **InsightFace** with ONNX Runtime for fast, accurate face embedding extraction and comparison.

## Features

- **Face Embedding**: Extract 512-dimensional face embeddings from images
- **Face Comparison**: Compare two face images for identity verification
- **Multi-face Detection**: Detects all faces, uses the most prominent one
- **Confidence Scores**: Returns detection confidence for quality filtering
- **GPU Support**: Optional CUDA acceleration

## API Endpoints

### POST /embed
Extract face embedding from an image URL.

**Request:**
```json
{
  "image_url": "https://example.com/photo.jpg"
}
```

**Response:**
```json
{
  "embedding": [0.123, -0.456, ...],  // 512 floats
  "score": 0.98,                       // Detection confidence
  "faces_detected": 1
}
```

### POST /compare
Compare two face images.

**Request:**
```json
{
  "image_url_1": "https://example.com/photo1.jpg",
  "image_url_2": "https://example.com/photo2.jpg"
}
```

**Response:**
```json
{
  "similarity": 0.85,
  "match": true,
  "threshold": 0.5
}
```

### GET /health
Health check endpoint.

## Running Locally

```bash
# Install dependencies
pip install -r requirements.txt

# Run the service
python main.py
# Or with uvicorn directly
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
```

## Running with Docker

```bash
# Build the image
docker build -t face-service .

# Run the container
docker run -p 8000:8000 face-service

# With GPU support (requires nvidia-docker)
docker run --gpus all -p 8000:8000 face-service
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `PORT` | 8000 | Server port |
| `MATCH_THRESHOLD` | 0.5 | Similarity threshold for face matching |

## Model Details

Uses InsightFace's **buffalo_l** model:
- Detection: RetinaFace with ResNet-50 backbone
- Recognition: ArcFace with ResNet-100 backbone
- Embedding size: 512 dimensions
- Model size: ~300MB (downloaded on first run)

## GPU Acceleration

For CUDA support, change the provider in `main.py`:

```python
face_model = FaceAnalysis(
    name='buffalo_l',
    providers=['CUDAExecutionProvider', 'CPUExecutionProvider']
)
```

And install CUDA-enabled ONNX Runtime:
```bash
pip install onnxruntime-gpu
```

## Testing

```bash
# Test embedding
curl -X POST http://localhost:8000/embed \
  -H "Content-Type: application/json" \
  -d '{"image_url": "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a7/Camponotus_flavomarginatus_ant.jpg/640px-Camponotus_flavomarginatus_ant.jpg"}'

# Test health
curl http://localhost:8000/health
```

## Integration with Attendance Engine

The Go worker calls this service via HTTP:

1. Check-in event created → queued in Redis
2. Worker picks up event → calls `/embed` endpoint
3. Face service returns embedding + confidence score
4. Worker updates event status with score

Future enhancements:
- Store user face embeddings in database
- Compare against stored embeddings for verification
- Add `/verify` endpoint for user identity confirmation
