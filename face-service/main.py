# Face Recognition Microservice
# Uses InsightFace with ONNX Runtime for powerful face recognition
# Features: Enrollment, 1:N search, quality assessment, liveness detection

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field
from typing import Optional
import numpy as np
import requests
from io import BytesIO
from PIL import Image
import os
import json
import base64
import re
from datetime import datetime
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="Face Recognition Service",
    description="Powerful face recognition with enrollment, search, and anti-spoofing",
    version="2.0.0"
)

# Enable CORS for web frontend
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],  # Allow all origins in dev; restrict in production
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Configuration
MATCH_THRESHOLD = float(os.getenv("MATCH_THRESHOLD", "0.45"))
QUALITY_THRESHOLD = float(os.getenv("QUALITY_THRESHOLD", "0.3"))
DETECTION_SIZE = int(os.getenv("DETECTION_SIZE", "640"))
USE_GPU = os.getenv("USE_GPU", "false").lower() == "true"
REDIS_URL = os.getenv("REDIS_URL", "")

# Model loading (lazy initialization)
face_model = None
redis_client = None


def get_redis():
    """Get Redis connection for persistent embedding storage."""
    global redis_client
    if redis_client is None and REDIS_URL:
        try:
            import redis
            redis_client = redis.from_url(REDIS_URL, decode_responses=False)
            redis_client.ping()
            logger.info("Redis connected for face gallery storage")
        except Exception as e:
            logger.warning(f"Redis unavailable, using in-memory storage: {e}")
            redis_client = "unavailable"
    return redis_client if redis_client != "unavailable" else None


# In-memory face gallery (fallback when Redis unavailable)
face_gallery: dict[str, dict] = {}


def get_face_model():
    """Lazy load the face recognition model."""
    global face_model
    if face_model is None:
        try:
            from insightface.app import FaceAnalysis
            providers = ['CUDAExecutionProvider', 'CPUExecutionProvider'] if USE_GPU else ['CPUExecutionProvider']
            face_model = FaceAnalysis(
                name='buffalo_l',  # High accuracy model (300MB)
                providers=providers,
                allowed_modules=['detection', 'recognition', 'genderage']
            )
            face_model.prepare(ctx_id=0 if USE_GPU else -1, det_size=(DETECTION_SIZE, DETECTION_SIZE))
            logger.info(f"Face model loaded (GPU: {USE_GPU})")
        except ImportError:
            logger.warning("InsightFace not installed. Using mock embeddings.")
            face_model = "mock"
    return face_model


# ============ Pydantic Models ============

class FaceQuality(BaseModel):
    """Face quality metrics."""
    score: float = Field(..., description="Overall quality score 0-1")
    blur: float = Field(..., description="Blur score (lower is sharper)")
    pose_yaw: float = Field(..., description="Head yaw angle in degrees")
    pose_pitch: float = Field(..., description="Head pitch angle in degrees")
    pose_roll: float = Field(..., description="Head roll angle in degrees")
    face_size: int = Field(..., description="Face bounding box area in pixels")
    is_frontal: bool = Field(..., description="Whether face is roughly frontal")


class EmbedRequest(BaseModel):
    image_url: str


class EmbedResponse(BaseModel):
    embedding: list[float]
    score: float = Field(..., description="Detection confidence")
    faces_detected: int
    quality: Optional[FaceQuality] = None


class CompareRequest(BaseModel):
    image_url_1: str
    image_url_2: str


class CompareResponse(BaseModel):
    similarity: float
    match: bool
    threshold: float
    quality_1: Optional[FaceQuality] = None
    quality_2: Optional[FaceQuality] = None


class EnrollRequest(BaseModel):
    user_id: str = Field(..., description="Unique user identifier")
    image_url: str = Field(..., description="Face image URL")
    name: Optional[str] = Field(None, description="Optional display name")
    metadata: Optional[dict] = Field(None, description="Optional metadata")


class EnrollResponse(BaseModel):
    user_id: str
    success: bool
    quality: Optional[FaceQuality] = None
    message: str


class SearchRequest(BaseModel):
    image_url: str
    top_k: int = Field(5, description="Number of top matches to return", ge=1, le=100)
    threshold: Optional[float] = Field(None, description="Custom similarity threshold")


class SearchMatch(BaseModel):
    user_id: str
    similarity: float
    name: Optional[str] = None


class SearchResponse(BaseModel):
    matches: list[SearchMatch]
    faces_detected: int
    quality: Optional[FaceQuality] = None


class VerifyRequest(BaseModel):
    user_id: str
    image_url: str


class VerifyResponse(BaseModel):
    user_id: str
    verified: bool
    similarity: float
    threshold: float
    quality: Optional[FaceQuality] = None


class LivenessRequest(BaseModel):
    image_url: str


class LivenessResponse(BaseModel):
    is_live: bool
    confidence: float
    checks: dict


class BatchEmbedRequest(BaseModel):
    image_urls: list[str] = Field(..., max_length=20)


class BatchEmbedResult(BaseModel):
    image_url: str
    success: bool
    embedding: Optional[list[float]] = None
    score: Optional[float] = None
    error: Optional[str] = None


class BatchEmbedResponse(BaseModel):
    results: list[BatchEmbedResult]


class IdentifyRequest(BaseModel):
    """Request for identifying a face from camera capture."""
    image_data: str = Field(..., description="Base64 encoded image (data:image/jpeg;base64,... or raw base64)")
    top_k: int = Field(3, description="Number of top matches to return", ge=1, le=20)


class IdentifyResponse(BaseModel):
    """Response from identify endpoint."""
    identified: bool = Field(..., description="Whether a match was found above threshold")
    matches: list[SearchMatch]
    best_match: Optional[SearchMatch] = None
    quality: Optional[FaceQuality] = None
    liveness: Optional[dict] = None


# ============ Helper Functions ============

def download_image(url: str) -> Image.Image:
    """Download image from URL with timeout and size limit."""
    try:
        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        }
        response = requests.get(url, timeout=15, stream=True, headers=headers, verify=True)
        response.raise_for_status()
        
        # Limit to 10MB
        content = b""
        for chunk in response.iter_content(chunk_size=8192):
            content += chunk
            if len(content) > 10 * 1024 * 1024:
                raise HTTPException(status_code=400, detail="Image too large (max 10MB)")
        
        img = Image.open(BytesIO(content))
        # Convert to RGB, handle various formats
        if img.mode in ('RGBA', 'LA', 'P'):
            background = Image.new('RGB', img.size, (255, 255, 255))
            if img.mode == 'P':
                img = img.convert('RGBA')
            background.paste(img, mask=img.split()[-1] if 'A' in img.mode else None)
            img = background
        elif img.mode != 'RGB':
            img = img.convert('RGB')
        
        return img
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Failed to download image: {str(e)}")


def parse_base64_image(image_data: str) -> Image.Image:
    """Parse base64 encoded image data."""
    try:
        # Handle data URL format: data:image/jpeg;base64,/9j/4AAQ...
        if image_data.startswith('data:'):
            # Extract the base64 part after the comma
            match = re.match(r'data:image/[^;]+;base64,(.+)', image_data)
            if match:
                image_data = match.group(1)
            else:
                raise HTTPException(status_code=400, detail="Invalid data URL format")
        
        # Decode base64
        image_bytes = base64.b64decode(image_data)
        
        if len(image_bytes) > 10 * 1024 * 1024:
            raise HTTPException(status_code=400, detail="Image too large (max 10MB)")
        
        img = Image.open(BytesIO(image_bytes))
        
        # Convert to RGB
        if img.mode in ('RGBA', 'LA', 'P'):
            background = Image.new('RGB', img.size, (255, 255, 255))
            if img.mode == 'P':
                img = img.convert('RGBA')
            background.paste(img, mask=img.split()[-1] if 'A' in img.mode else None)
            img = background
        elif img.mode != 'RGB':
            img = img.convert('RGB')
        
        return img
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Failed to parse image: {str(e)}")


def calculate_blur(img_array: np.ndarray, bbox: tuple) -> float:
    """Calculate blur score using Laplacian variance."""
    x1, y1, x2, y2 = map(int, bbox)
    face_region = img_array[y1:y2, x1:x2]
    
    if face_region.size == 0:
        return 1.0
    
    # Convert to grayscale
    if len(face_region.shape) == 3:
        gray = np.mean(face_region, axis=2).astype(np.uint8)
    else:
        gray = face_region
    
    # Laplacian variance (higher = sharper)
    try:
        from scipy import ndimage
        laplacian = ndimage.laplace(gray.astype(float))
        variance = laplacian.var()
        # Normalize: higher variance = better quality (less blur)
        blur_score = min(variance / 500, 1.0)
        return 1.0 - blur_score  # Return blur level (0 = sharp, 1 = blurry)
    except ImportError:
        # Fallback: use simple gradient magnitude
        gy, gx = np.gradient(gray.astype(float))
        gnorm = np.sqrt(gx**2 + gy**2)
        sharpness = np.mean(gnorm)
        return max(0, 1.0 - (sharpness / 30))


def assess_face_quality(face, img_array: np.ndarray) -> FaceQuality:
    """Assess face quality from InsightFace detection result."""
    bbox = face.bbox
    
    # Calculate face size
    face_width = bbox[2] - bbox[0]
    face_height = bbox[3] - bbox[1]
    face_size = int(face_width * face_height)
    
    # Get pose angles (if available)
    pose_yaw, pose_pitch, pose_roll = 0.0, 0.0, 0.0
    if hasattr(face, 'pose') and face.pose is not None:
        pose_yaw, pose_pitch, pose_roll = face.pose[:3]
    
    # Calculate blur
    blur = calculate_blur(img_array, bbox)
    
    # Check if frontal (within angle thresholds)
    is_frontal = bool(abs(pose_yaw) < 30 and abs(pose_pitch) < 25 and abs(pose_roll) < 20)

    # Overall quality score
    det_score = float(face.det_score)
    pose_penalty = (abs(pose_yaw) + abs(pose_pitch) + abs(pose_roll)) / 180  # 0-1
    blur_penalty = blur
    size_score = min(face_size / (200 * 200), 1.0)  # Ideal: 200x200 face
    
    quality_score = det_score * 0.3 + (1 - pose_penalty) * 0.25 + (1 - blur_penalty) * 0.25 + size_score * 0.2
    
    return FaceQuality(
        score=round(quality_score, 3),
        blur=round(blur, 3),
        pose_yaw=round(float(pose_yaw), 1),
        pose_pitch=round(float(pose_pitch), 1),
        pose_roll=round(float(pose_roll), 1),
        face_size=face_size,
        is_frontal=is_frontal
    )


def get_best_face(faces: list, img_array: np.ndarray):
    """Select the best face based on quality metrics."""
    if len(faces) == 1:
        return faces[0]
    
    # Score each face
    scored_faces = []
    for face in faces:
        quality = assess_face_quality(face, img_array)
        scored_faces.append((face, quality.score))
    
    # Return face with highest quality
    scored_faces.sort(key=lambda x: x[1], reverse=True)
    return scored_faces[0][0]


def get_embedding(image: Image.Image, return_quality: bool = True) -> tuple:
    """Extract face embedding from image with quality assessment."""
    model = get_face_model()
    
    if model == "mock":
        # Return mock embedding for testing
        mock_quality = FaceQuality(
            score=0.85, blur=0.1, pose_yaw=5.0, pose_pitch=3.0,
            pose_roll=1.0, face_size=40000, is_frontal=True
        )
        return [float(x) for x in np.random.rand(512).tolist()], 0.95, 1, mock_quality
    
    # Convert PIL to numpy array
    img_array = np.array(image)
    
    # Detect faces and get embeddings
    faces = model.get(img_array)
    
    if len(faces) == 0:
        raise HTTPException(status_code=400, detail="No face detected in image")
    
    # Get the best quality face
    face = get_best_face(faces, img_array)
    
    embedding = face.embedding.tolist()
    det_score = float(face.det_score)
    quality = assess_face_quality(face, img_array) if return_quality else None
    
    return embedding, det_score, len(faces), quality


def cosine_similarity(emb1: list[float], emb2: list[float]) -> float:
    """Calculate cosine similarity between two embeddings."""
    a = np.array(emb1)
    b = np.array(emb2)
    dot = np.dot(a, b)
    norm_a = np.linalg.norm(a)
    norm_b = np.linalg.norm(b)
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return float(dot / (norm_a * norm_b))


def save_to_gallery(user_id: str, embedding: list[float], name: str = None, metadata: dict = None):
    """Save face embedding to gallery (Redis or in-memory)."""
    data = {
        "embedding": embedding,
        "name": name,
        "metadata": metadata or {},
        "enrolled_at": datetime.utcnow().isoformat()
    }
    
    r = get_redis()
    if r:
        r.hset("face_gallery", user_id, json.dumps(data, default=lambda x: x.tolist() if isinstance(x, np.ndarray) else x).encode())
    else:
        face_gallery[user_id] = data


def load_from_gallery(user_id: str) -> Optional[dict]:
    """Load face embedding from gallery."""
    r = get_redis()
    if r:
        data = r.hget("face_gallery", user_id)
        if data:
            return json.loads(data)
        return None
    return face_gallery.get(user_id)


def get_all_gallery() -> dict[str, dict]:
    """Get all enrolled faces."""
    r = get_redis()
    if r:
        all_data = r.hgetall("face_gallery")
        return {k.decode() if isinstance(k, bytes) else k: json.loads(v) for k, v in all_data.items()}
    return face_gallery.copy()


def delete_from_gallery(user_id: str) -> bool:
    """Delete face from gallery."""
    r = get_redis()
    if r:
        return r.hdel("face_gallery", user_id) > 0
    if user_id in face_gallery:
        del face_gallery[user_id]
        return True
    return False


def check_liveness(face, img_array: np.ndarray) -> tuple[bool, float, dict]:
    """
    Basic liveness detection checks.
    
    Current implementation uses heuristic checks:
    1. Face size and proportion check
    2. Texture analysis (screen vs real skin)
    3. Color distribution analysis
    4. Edge sharpness (printed photos have different edges)
    """
    checks = {}
    
    bbox = face.bbox.astype(int)
    x1, y1, x2, y2 = bbox
    face_region = img_array[max(0, y1):min(img_array.shape[0], y2), 
                            max(0, x1):min(img_array.shape[1], x2)]
    
    if face_region.size == 0:
        return False, 0.0, {"error": "Invalid face region"}
    
    # 1. Screen moire pattern detection (screens have periodic patterns)
    gray = np.mean(face_region, axis=2) if len(face_region.shape) == 3 else face_region
    fft = np.fft.fft2(gray)
    fft_shift = np.fft.fftshift(fft)
    magnitude = np.abs(fft_shift)
    
    # High frequency ratio (screens have more high freq noise)
    h, w = magnitude.shape
    center_h, center_w = h // 2, w // 2
    low_freq = np.mean(magnitude[center_h-10:center_h+10, center_w-10:center_w+10])
    high_freq = np.mean(magnitude) - low_freq
    freq_ratio = high_freq / (low_freq + 1e-6)
    screen_score = 1.0 - min(freq_ratio / 2, 1.0)
    checks["screen_pattern"] = round(screen_score, 3)
    
    # 2. Color distribution (real skin has warmer, varied tones)
    if len(face_region.shape) == 3:
        r_mean, g_mean, b_mean = np.mean(face_region, axis=(0, 1))
        r_std, g_std, b_std = np.std(face_region, axis=(0, 1))
        
        # Real skin typically: R > G > B, with variation
        color_natural = 1.0 if (r_mean > g_mean > b_mean) else 0.6
        color_variance = min((r_std + g_std + b_std) / 150, 1.0)  # Natural skin has variance
        color_score = (color_natural * 0.5 + color_variance * 0.5)
        checks["color_distribution"] = round(color_score, 3)
    else:
        color_score = 0.5
        checks["color_distribution"] = 0.5
    
    # 3. Face proportion check (printed photos often have unnatural proportions)
    face_width = x2 - x1
    face_height = y2 - y1
    aspect_ratio = face_width / (face_height + 1e-6)
    # Natural face aspect ratio is typically 0.65-0.85
    proportion_score = 1.0 if 0.6 < aspect_ratio < 0.9 else 0.5
    checks["face_proportion"] = round(proportion_score, 3)
    
    # 4. Detection confidence (lower confidence often means spoofed)
    det_score = float(face.det_score)
    checks["detection_confidence"] = round(det_score, 3)
    
    # 5. Texture complexity (real faces have more micro-textures)
    if len(face_region.shape) == 3:
        gray_face = np.mean(face_region, axis=2)
    else:
        gray_face = face_region
    
    # Local binary pattern approximation
    texture_grad = np.mean(np.abs(np.diff(gray_face, axis=0))) + np.mean(np.abs(np.diff(gray_face, axis=1)))
    texture_score = min(texture_grad / 25, 1.0)
    checks["texture_complexity"] = round(texture_score, 3)
    
    # Combined liveness score
    weights = {
        "screen_pattern": 0.25,
        "color_distribution": 0.2,
        "face_proportion": 0.1,
        "detection_confidence": 0.25,
        "texture_complexity": 0.2
    }
    
    confidence = sum(checks.get(k, 0.5) * w for k, w in weights.items())
    is_live = bool(confidence > 0.55)

    return is_live, round(float(confidence), 3), checks

# ============ API Endpoints ============

@app.get("/health")
async def health():
    """Health check endpoint."""
    model = get_face_model()
    r = get_redis()
    
    gallery_count = 0
    if r:
        gallery_count = r.hlen("face_gallery")
    else:
        gallery_count = len(face_gallery)
    
    return {
        "status": "ok",
        "model_loaded": model is not None and model != "mock",
        "model_name": "buffalo_l" if model and model != "mock" else "mock",
        "gpu_enabled": USE_GPU,
        "redis_connected": r is not None,
        "gallery_size": gallery_count,
        "match_threshold": MATCH_THRESHOLD
    }


@app.post("/embed", response_model=EmbedResponse)
async def embed(request: EmbedRequest):
    """
    Extract face embedding from an image URL.
    
    Returns 512-dimensional face embedding with quality metrics.
    """
    image = download_image(request.image_url)
    embedding, score, faces, quality = get_embedding(image)
    
    return EmbedResponse(
        embedding=embedding,
        score=score,
        faces_detected=faces,
        quality=quality
    )


@app.post("/compare", response_model=CompareResponse)
async def compare(request: CompareRequest):
    """
    Compare two face images and determine if they match.
    
    Returns similarity score and match decision.
    """
    image1 = download_image(request.image_url_1)
    image2 = download_image(request.image_url_2)
    
    emb1, _, _, quality1 = get_embedding(image1)
    emb2, _, _, quality2 = get_embedding(image2)
    
    similarity = cosine_similarity(emb1, emb2)
    
    return CompareResponse(
        similarity=round(similarity, 4),
        match=similarity >= MATCH_THRESHOLD,
        threshold=MATCH_THRESHOLD,
        quality_1=quality1,
        quality_2=quality2
    )


@app.post("/enroll", response_model=EnrollResponse)
async def enroll(request: EnrollRequest):
    """
    Enroll a new face into the recognition gallery.
    
    The face will be stored and available for 1:N search.
    Existing enrollment for same user_id will be updated.
    """
    image = download_image(request.image_url)
    
    try:
        embedding, score, faces, quality = get_embedding(image)
    except HTTPException as e:
        return EnrollResponse(
            user_id=request.user_id,
            success=False,
            quality=None,
            message=str(e.detail)
        )
    
    # Check quality threshold
    if quality and quality.score < QUALITY_THRESHOLD:
        return EnrollResponse(
            user_id=request.user_id,
            success=False,
            quality=quality,
            message=f"Face quality too low ({quality.score:.2f} < {QUALITY_THRESHOLD}). Please use a clearer, frontal photo."
        )
    
    # Save to gallery
    save_to_gallery(request.user_id, embedding, request.name, request.metadata)
    
    return EnrollResponse(
        user_id=request.user_id,
        success=True,
        quality=quality,
        message="Face enrolled successfully"
    )


@app.delete("/enroll/{user_id}")
async def unenroll(user_id: str):
    """Remove a face from the gallery."""
    deleted = delete_from_gallery(user_id)
    if not deleted:
        raise HTTPException(status_code=404, detail=f"User {user_id} not found in gallery")
    return {"user_id": user_id, "deleted": True}


@app.get("/gallery")
async def list_gallery():
    """List all enrolled users (without embeddings)."""
    gallery = get_all_gallery()
    users = []
    for user_id, data in gallery.items():
        users.append({
            "user_id": user_id,
            "name": data.get("name"),
            "enrolled_at": data.get("enrolled_at"),
            "metadata": data.get("metadata", {})
        })
    return {"users": users, "total": len(users)}


@app.post("/search", response_model=SearchResponse)
async def search(request: SearchRequest):
    """
    Search for matching faces in the enrolled gallery (1:N identification).
    
    Returns top-k matches above threshold.
    """
    image = download_image(request.image_url)
    embedding, score, faces, quality = get_embedding(image)
    
    threshold = request.threshold if request.threshold is not None else MATCH_THRESHOLD
    gallery = get_all_gallery()
    
    if not gallery:
        return SearchResponse(matches=[], faces_detected=faces, quality=quality)
    
    # Calculate similarity with all enrolled faces
    matches = []
    for user_id, data in gallery.items():
        stored_emb = data["embedding"]
        similarity = cosine_similarity(embedding, stored_emb)
        
        if similarity >= threshold:
            matches.append(SearchMatch(
                user_id=user_id,
                similarity=round(similarity, 4),
                name=data.get("name")
            ))
    
    # Sort by similarity descending and take top_k
    matches.sort(key=lambda x: x.similarity, reverse=True)
    matches = matches[:request.top_k]
    
    return SearchResponse(
        matches=matches,
        faces_detected=faces,
        quality=quality
    )


@app.post("/identify", response_model=IdentifyResponse)
async def identify(request: IdentifyRequest):
    """
    Identify a person from a camera-captured image (base64).
    
    This endpoint is designed for live camera capture:
    - Accepts base64 encoded image (from canvas.toDataURL())
    - Searches against enrolled faces
    - Returns best match with quality and liveness info
    """
    # Parse base64 image
    image = parse_base64_image(request.image_data)
    
    model = get_face_model()
    img_array = np.array(image)
    
    if model == "mock":
        return IdentifyResponse(
            identified=True,
            matches=[SearchMatch(user_id="mock-user", similarity=0.95, name="Mock User")],
            best_match=SearchMatch(user_id="mock-user", similarity=0.95, name="Mock User"),
            quality=FaceQuality(score=0.85, blur=0.1, pose_yaw=0.0, pose_pitch=0.0, pose_roll=0.0, face_size=40000, is_frontal=True),
            liveness={"is_live": True, "confidence": 0.9}
        )
    
    # Detect faces
    faces = model.get(img_array)
    
    if len(faces) == 0:
        raise HTTPException(status_code=400, detail="No face detected in image")
    
    # Get best quality face
    face = get_best_face(faces, img_array)
    embedding = face.embedding.tolist()
    quality = assess_face_quality(face, img_array)
    
    # Check liveness
    is_live, live_conf, live_checks = check_liveness(face, img_array)
    liveness_result = {
        "is_live": is_live,
        "confidence": live_conf,
        "checks": live_checks
    }
    
    # Search gallery
    gallery = get_all_gallery()
    
    if not gallery:
        return IdentifyResponse(
            identified=False,
            matches=[],
            best_match=None,
            quality=quality,
            liveness=liveness_result
        )
    
    # Calculate similarity with all enrolled faces
    matches = []
    for user_id, data in gallery.items():
        stored_emb = data["embedding"]
        similarity = cosine_similarity(embedding, stored_emb)
        
        if similarity >= MATCH_THRESHOLD * 0.8:  # Include slightly lower matches for display
            matches.append(SearchMatch(
                user_id=user_id,
                similarity=round(similarity, 4),
                name=data.get("name")
            ))
    
    # Sort by similarity
    matches.sort(key=lambda x: x.similarity, reverse=True)
    matches = matches[:request.top_k]
    
    # Determine if identified
    best_match = matches[0] if matches and matches[0].similarity >= MATCH_THRESHOLD else None
    identified = best_match is not None
    
    return IdentifyResponse(
        identified=identified,
        matches=matches,
        best_match=best_match,
        quality=quality,
        liveness=liveness_result
    )


@app.post("/verify", response_model=VerifyResponse)
async def verify(request: VerifyRequest):
    """
    Verify a face against a specific enrolled user (1:1 verification).
    
    Returns whether the face matches the enrolled user.
    """
    # Load enrolled embedding
    stored = load_from_gallery(request.user_id)
    if not stored:
        raise HTTPException(status_code=404, detail=f"User {request.user_id} not enrolled")
    
    image = download_image(request.image_url)
    embedding, score, faces, quality = get_embedding(image)
    
    similarity = cosine_similarity(embedding, stored["embedding"])
    
    return VerifyResponse(
        user_id=request.user_id,
        verified=similarity >= MATCH_THRESHOLD,
        similarity=round(similarity, 4),
        threshold=MATCH_THRESHOLD,
        quality=quality
    )


@app.post("/liveness", response_model=LivenessResponse)
async def liveness(request: LivenessRequest):
    """
    Check if the face image is from a live person (anti-spoofing).
    
    Detects presentation attacks like:
    - Printed photos
    - Screen replay attacks
    - Masks (basic detection)
    """
    model = get_face_model()
    
    if model == "mock":
        return LivenessResponse(
            is_live=True,
            confidence=0.85,
            checks={"mock": True}
        )
    
    image = download_image(request.image_url)
    img_array = np.array(image)
    
    faces = model.get(img_array)
    if len(faces) == 0:
        raise HTTPException(status_code=400, detail="No face detected")
    
    face = get_best_face(faces, img_array)
    is_live, confidence, checks = check_liveness(face, img_array)
    
    return LivenessResponse(
        is_live=is_live,
        confidence=confidence,
        checks=checks
    )


@app.post("/batch/embed", response_model=BatchEmbedResponse)
async def batch_embed(request: BatchEmbedRequest):
    """
    Extract embeddings from multiple images in a single request.
    
    More efficient than multiple single requests.
    """
    results = []
    
    for url in request.image_urls:
        try:
            image = download_image(url)
            embedding, score, _, _ = get_embedding(image, return_quality=False)
            results.append(BatchEmbedResult(
                image_url=url,
                success=True,
                embedding=embedding,
                score=score
            ))
        except Exception as e:
            results.append(BatchEmbedResult(
                image_url=url,
                success=False,
                error=str(e)
            ))
    
    return BatchEmbedResponse(results=results)


@app.post("/analyze")
async def analyze(image_url: str):
    """
    Full face analysis including detection, quality, attributes, and liveness.
    
    Comprehensive endpoint for face inspection.
    """
    model = get_face_model()
    image = download_image(image_url)
    img_array = np.array(image)
    
    if model == "mock":
        return {
            "faces_detected": 1,
            "faces": [{
                "bbox": [100, 100, 300, 350],
                "detection_score": 0.95,
                "quality": {"score": 0.85, "is_frontal": True},
                "age": 30,
                "gender": "M",
                "liveness": {"is_live": True, "confidence": 0.85}
            }]
        }
    
    faces = model.get(img_array)
    
    if len(faces) == 0:
        return {"faces_detected": 0, "faces": []}
    
    results = []
    for face in faces:
        quality = assess_face_quality(face, img_array)
        is_live, live_conf, live_checks = check_liveness(face, img_array)
        
        face_data = {
            "bbox": face.bbox.tolist(),
            "detection_score": round(float(face.det_score), 3),
            "quality": quality.model_dump(),
            "liveness": {
                "is_live": is_live,
                "confidence": live_conf,
                "checks": live_checks
            }
        }
        
        # Add age/gender if available
        if hasattr(face, 'age'):
            face_data["age"] = int(face.age)
        if hasattr(face, 'gender'):
            face_data["gender"] = "M" if face.gender == 1 else "F"
        
        results.append(face_data)
    
    return {
        "faces_detected": len(faces),
        "faces": results
    }


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8000"))
    logger.info(f"Starting Face Recognition Service on port {port}")
    logger.info(f"GPU: {USE_GPU}, Match Threshold: {MATCH_THRESHOLD}")
    uvicorn.run(app, host="0.0.0.0", port=port)
