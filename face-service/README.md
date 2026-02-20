# Face Recognition Microservice

A powerful FastAPI-based face recognition service using **InsightFace** with ONNX Runtime. Features face enrollment, 1:N identification, 1:1 verification, quality assessment, and liveness detection.

## Features

- **Face Embedding**: Extract 512-dimensional face embeddings from images
- **Face Comparison**: Compare two face images for identity verification
- **Face Enrollment**: Store faces in a gallery for 1:N search
- **1:N Face Search**: Find matching faces from enrolled gallery
- **1:1 Verification**: Verify a face against a specific enrolled user
- **Quality Assessment**: Measure blur, pose angles, and overall quality
- **Liveness Detection**: Anti-spoofing checks for presentation attacks
- **Batch Processing**: Process multiple images efficiently
- **Multi-face Detection**: Detects all faces, selects the best quality one
- **Redis Persistence**: Optionally persist face gallery to Redis
- **GPU Support**: Optional CUDA acceleration

## API Endpoints

### POST /embed
Extract face embedding with quality metrics.

```json
// Request
{ "image_url": "https://example.com/photo.jpg" }

// Response
{
  "embedding": [0.123, -0.456, ...],  // 512 floats
  "score": 0.98,
  "faces_detected": 1,
  "quality": {
    "score": 0.85,
    "blur": 0.1,
    "pose_yaw": 5.0,
    "pose_pitch": 3.0,
    "pose_roll": 1.0,
    "face_size": 40000,
    "is_frontal": true
  }
}
```

### POST /compare
Compare two face images.

```json
// Request
{
  "image_url_1": "https://example.com/photo1.jpg",
  "image_url_2": "https://example.com/photo2.jpg"
}

// Response
{
  "similarity": 0.85,
  "match": true,
  "threshold": 0.45,
  "quality_1": { ... },
  "quality_2": { ... }
}
```

### POST /enroll
Enroll a face into the recognition gallery.

```json
// Request
{
  "user_id": "user123",
  "image_url": "https://example.com/photo.jpg",
  "name": "John Doe",
  "metadata": {"department": "Engineering"}
}

// Response
{
  "user_id": "user123",
  "success": true,
  "quality": { ... },
  "message": "Face enrolled successfully"
}
```

### DELETE /enroll/{user_id}
Remove a face from the gallery.

### GET /gallery
List all enrolled users (without embeddings).

### POST /search
1:N face identification against enrolled gallery.

```json
// Request
{
  "image_url": "https://example.com/photo.jpg",
  "top_k": 5,
  "threshold": 0.45
}

// Response
{
  "matches": [
    {"user_id": "user123", "similarity": 0.92, "name": "John Doe"},
    {"user_id": "user456", "similarity": 0.67, "name": "Jane Smith"}
  ],
  "faces_detected": 1,
  "quality": { ... }
}
```

### POST /verify
1:1 verification against specific enrolled user.

```json
// Request
{
  "user_id": "user123",
  "image_url": "https://example.com/photo.jpg"
}

// Response
{
  "user_id": "user123",
  "verified": true,
  "similarity": 0.92,
  "threshold": 0.45,
  "quality": { ... }
}
```

### POST /liveness
Anti-spoofing liveness detection.

```json
// Request
{ "image_url": "https://example.com/photo.jpg" }

// Response
{
  "is_live": true,
  "confidence": 0.78,
  "checks": {
    "screen_pattern": 0.85,
    "color_distribution": 0.72,
    "face_proportion": 1.0,
    "detection_confidence": 0.95,
    "texture_complexity": 0.68
  }
}
```

### POST /batch/embed
Batch extract embeddings from multiple images.

```json
// Request
{ "image_urls": ["url1", "url2", "url3"] }

// Response
{
  "results": [
    {"image_url": "url1", "success": true, "embedding": [...], "score": 0.95},
    {"image_url": "url2", "success": false, "error": "No face detected"}
  ]
}
```

### POST /analyze
Full face analysis with all metrics.

```json
// Response
{
  "faces_detected": 2,
  "faces": [
    {
      "bbox": [100, 100, 300, 350],
      "detection_score": 0.95,
      "quality": { ... },
      "age": 30,
      "gender": "M",
      "liveness": {"is_live": true, "confidence": 0.85, "checks": {...}}
    }
  ]
}
```

### GET /health
Health check with service status.

## Running Locally

```bash
# Install dependencies
pip install -r requirements.txt

# Run the service
python main.py

# Or with uvicorn
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
```

## Running with Docker

```bash
# Build the image
docker build -t face-service .

# Run the container
docker run -p 8000:8000 face-service

# With Redis for persistent gallery
docker run -p 8000:8000 -e REDIS_URL=redis://host:6379 face-service

# With GPU support (requires nvidia-docker)
docker run --gpus all -p 8000:8000 -e USE_GPU=true face-service
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `PORT` | 8000 | Server port |
| `MATCH_THRESHOLD` | 0.45 | Similarity threshold for matching (0-1) |
| `QUALITY_THRESHOLD` | 0.3 | Minimum quality for enrollment |
| `DETECTION_SIZE` | 640 | Face detection input size |
| `USE_GPU` | false | Enable CUDA GPU acceleration |
| `REDIS_URL` | "" | Redis URL for persistent gallery storage |

## Model Details

Uses InsightFace's **buffalo_l** model:
- **Detection**: RetinaFace with ResNet-50 backbone
- **Recognition**: ArcFace with ResNet-100 backbone
- **Attributes**: Gender and age estimation
- **Embedding size**: 512 dimensions
- **Model size**: ~300MB (downloaded on first run)
- **Accuracy**: 99.83% on LFW benchmark

## Quality Assessment

The quality score (0-1) is computed from:
- **Detection confidence** (30%): How certain the detector is
- **Pose angles** (25%): Yaw, pitch, roll penalty
- **Blur level** (25%): Laplacian variance measure
- **Face size** (20%): Larger faces score higher

Frontal faces (yaw < 30°, pitch < 25°, roll < 20°) are preferred.

## Liveness Detection

Heuristic-based anti-spoofing checks:
- **Screen pattern**: FFT analysis for moire patterns
- **Color distribution**: Skin tone naturalness
- **Face proportion**: Aspect ratio analysis
- **Texture complexity**: Gradient magnitude
- **Detection confidence**: Lower scores may indicate spoofing

For high-security applications, consider adding a dedicated anti-spoofing model (e.g., Silent Face Anti-Spoofing).

## GPU Acceleration

For CUDA support:

```bash
# Install CUDA-enabled ONNX Runtime
pip install onnxruntime-gpu

# Run with GPU
USE_GPU=true python main.py
```

## Testing

```bash
# Test embedding
curl -X POST http://localhost:8000/embed \
  -H "Content-Type: application/json" \
  -d '{"image_url": "https://example.com/face.jpg"}'

# Test enrollment
curl -X POST http://localhost:8000/enroll \
  -H "Content-Type: application/json" \
  -d '{"user_id": "john", "image_url": "https://example.com/john.jpg", "name": "John Doe"}'

# Test search
curl -X POST http://localhost:8000/search \
  -H "Content-Type: application/json" \
  -d '{"image_url": "https://example.com/unknown.jpg", "top_k": 3}'

# Test liveness
curl -X POST http://localhost:8000/liveness \
  -H "Content-Type: application/json" \
  -d '{"image_url": "https://example.com/face.jpg"}'

# Health check
curl http://localhost:8000/health
```

## Integration with Attendance Engine

The Go attendance service integrates via HTTP:

1. **Enrollment Flow**:
   - Admin uploads user photo → calls `/enroll`
   - Face embedding stored in gallery

2. **Check-in Flow**:
   - User takes selfie → check-in event created
   - Worker calls `/search` to identify user
   - Or calls `/verify` if user_id is known
   - Optionally calls `/liveness` for anti-spoofing
   - Event updated with match score and verification status

3. **Quality Gate**:
   - Low quality images rejected at enrollment
   - Quality metrics returned for UI feedback
