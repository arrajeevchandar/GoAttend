"""
GoAttend Face Recognition Service
- POST /register  : Register a face (save image for matching)
- POST /recognize : Recognize a face against registered faces
- GET  /health    : Health check
"""

import os
import shutil
import uuid
from pathlib import Path

from deepface import DeepFace
from fastapi import FastAPI, File, Form, UploadFile, HTTPException
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(title="GoAttend Face Service")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# Store registered face images: faces/<student_id>.jpg
FACES_DIR = Path(os.getenv("FACES_DIR", "./faces"))
FACES_DIR.mkdir(parents=True, exist_ok=True)

MODEL_NAME = os.getenv("MODEL_NAME", "VGG-Face")
DETECTOR = os.getenv("DETECTOR", "opencv")
DISTANCE_METRIC = os.getenv("DISTANCE_METRIC", "cosine")
THRESHOLD = float(os.getenv("THRESHOLD", "0.40"))


@app.get("/health")
def health():
    registered = len(list(FACES_DIR.glob("*.jpg")))
    return {"status": "ok", "registered_faces": registered, "model": MODEL_NAME}


@app.post("/register")
async def register_face(
    student_id: str = Form(...),
    photo: UploadFile = File(...),
):
    """Save face image for a student. Validates that a face is detectable."""
    tmp_path = f"/tmp/{uuid.uuid4()}.jpg"
    with open(tmp_path, "wb") as f:
        shutil.copyfileobj(photo.file, f)

    # Verify a face exists in the image
    try:
        faces = DeepFace.extract_faces(
            img_path=tmp_path,
            detector_backend=DETECTOR,
            enforce_detection=True,
        )
        if not faces:
            os.remove(tmp_path)
            raise HTTPException(status_code=400, detail="No face detected")
    except ValueError as e:
        os.remove(tmp_path)
        raise HTTPException(status_code=400, detail=f"Face detection failed: {e}")

    # Save as <student_id>.jpg
    dest = FACES_DIR / f"{student_id}.jpg"
    shutil.move(tmp_path, str(dest))

    return {"status": "registered", "student_id": student_id}


@app.post("/recognize")
async def recognize_face(
    photo: UploadFile = File(...),
):
    """Compare uploaded face against all registered faces. Returns best match."""
    registered = list(FACES_DIR.glob("*.jpg"))
    if not registered:
        raise HTTPException(status_code=404, detail="No faces registered yet")

    tmp_path = f"/tmp/{uuid.uuid4()}.jpg"
    with open(tmp_path, "wb") as f:
        shutil.copyfileobj(photo.file, f)

    # Verify face in uploaded image
    try:
        DeepFace.extract_faces(
            img_path=tmp_path,
            detector_backend=DETECTOR,
            enforce_detection=True,
        )
    except ValueError:
        os.remove(tmp_path)
        raise HTTPException(status_code=400, detail="No face detected in image")

    best_match = None
    best_distance = float("inf")

    for face_path in registered:
        try:
            result = DeepFace.verify(
                img1_path=tmp_path,
                img2_path=str(face_path),
                model_name=MODEL_NAME,
                detector_backend=DETECTOR,
                distance_metric=DISTANCE_METRIC,
                enforce_detection=False,
            )
            distance = result["distance"]
            if distance < best_distance:
                best_distance = distance
                best_match = face_path.stem  # student_id (the DB uuid)
        except Exception:
            continue

    os.remove(tmp_path)

    if best_match and best_distance <= THRESHOLD:
        return {
            "matched": True,
            "student_id": best_match,
            "distance": round(best_distance, 4),
        }

    return {
        "matched": False,
        "distance": round(best_distance, 4) if best_distance < float("inf") else None,
    }


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("PORT", "8000"))
    uvicorn.run(app, host="0.0.0.0", port=port)
