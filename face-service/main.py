# Face Recognition Microservice
# Uses InsightFace with ONNX Runtime for fast face embedding

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import numpy as np
import requests
from io import BytesIO
from PIL import Image
import os

app = FastAPI(title="Face Recognition Service")

# Model loading (lazy initialization)
face_model = None

def get_face_model():
    """Lazy load the face recognition model."""
    global face_model
    if face_model is None:
        try:
            from insightface.app import FaceAnalysis
            face_model = FaceAnalysis(
                name='buffalo_l',  # High accuracy model
                providers=['CPUExecutionProvider']  # Use CUDAExecutionProvider for GPU
            )
            face_model.prepare(ctx_id=0, det_size=(640, 640))
            print("Face model loaded successfully")
        except ImportError:
            print("InsightFace not installed. Using mock embeddings.")
            face_model = "mock"
    return face_model


class EmbedRequest(BaseModel):
    image_url: str


class EmbedResponse(BaseModel):
    embedding: list[float]
    score: float
    faces_detected: int


class CompareRequest(BaseModel):
    image_url_1: str
    image_url_2: str


class CompareResponse(BaseModel):
    similarity: float
    match: bool
    threshold: float


def download_image(url: str) -> Image.Image:
    """Download image from URL."""
    try:
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        return Image.open(BytesIO(response.content)).convert('RGB')
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Failed to download image: {str(e)}")


def get_embedding(image: Image.Image) -> tuple[list[float], float, int]:
    """Extract face embedding from image."""
    model = get_face_model()
    
    if model == "mock":
        # Return mock embedding for testing
        return [float(x) for x in np.random.rand(512).tolist()], 0.95, 1
    
    # Convert PIL to numpy array
    img_array = np.array(image)
    
    # Detect faces and get embeddings
    faces = model.get(img_array)
    
    if len(faces) == 0:
        raise HTTPException(status_code=400, detail="No face detected in image")
    
    # Get the largest face (most prominent)
    face = max(faces, key=lambda x: (x.bbox[2] - x.bbox[0]) * (x.bbox[3] - x.bbox[1]))
    
    embedding = face.embedding.tolist()
    det_score = float(face.det_score)
    
    return embedding, det_score, len(faces)


def cosine_similarity(emb1: list[float], emb2: list[float]) -> float:
    """Calculate cosine similarity between two embeddings."""
    a = np.array(emb1)
    b = np.array(emb2)
    return float(np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b)))


@app.get("/health")
async def health():
    """Health check endpoint."""
    return {"status": "ok", "model_loaded": face_model is not None}


@app.post("/embed", response_model=EmbedResponse)
async def embed(request: EmbedRequest):
    """
    Extract face embedding from an image URL.
    
    Returns:
        - embedding: 512-dimensional face embedding vector
        - score: Face detection confidence (0-1)
        - faces_detected: Number of faces found in image
    """
    image = download_image(request.image_url)
    embedding, score, faces = get_embedding(image)
    
    return EmbedResponse(
        embedding=embedding,
        score=score,
        faces_detected=faces
    )


@app.post("/compare", response_model=CompareResponse)
async def compare(request: CompareRequest):
    """
    Compare two face images and determine if they match.
    
    Returns:
        - similarity: Cosine similarity score (0-1)
        - match: Whether faces match (similarity > threshold)
        - threshold: The threshold used for matching
    """
    image1 = download_image(request.image_url_1)
    image2 = download_image(request.image_url_2)
    
    emb1, _, _ = get_embedding(image1)
    emb2, _, _ = get_embedding(image2)
    
    similarity = cosine_similarity(emb1, emb2)
    threshold = float(os.getenv("MATCH_THRESHOLD", "0.5"))
    
    return CompareResponse(
        similarity=similarity,
        match=similarity >= threshold,
        threshold=threshold
    )


@app.post("/verify")
async def verify(user_id: str, image_url: str):
    """
    Verify a user's face against stored embedding.
    
    This endpoint would integrate with your database to:
    1. Fetch the stored embedding for user_id
    2. Compare with the provided image
    3. Return match result
    
    For now, returns mock response.
    """
    # TODO: Integrate with database to fetch stored embeddings
    image = download_image(image_url)
    embedding, score, faces = get_embedding(image)
    
    # Mock verification - in production, compare against stored embedding
    return {
        "user_id": user_id,
        "verified": True,
        "confidence": score,
        "embedding_size": len(embedding)
    }


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8000"))
    uvicorn.run(app, host="0.0.0.0", port=port)
