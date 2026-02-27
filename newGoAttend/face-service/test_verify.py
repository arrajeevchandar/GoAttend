"""
Quick script to test face verification between two images.
Update IMG1 and IMG2 paths below, then run:  python test_verify.py
"""

from deepface import DeepFace

# ---- CHANGE THESE PATHS ----
IMG1 = "./image5.jpg"
IMG2 = "./image7.jpg"
# -----------------------------

print(f"\n🔍 Comparing faces...")
print(f"   Image 1: {IMG1}")
print(f"   Image 2: {IMG2}\n")

try:
    result = DeepFace.verify(
        img1_path=IMG1,
        img2_path=IMG2,
        # model_name="VGG-Face",
        # detector_backend="opencv",
        # distance_metric="cosine",
        # enforce_detection=True,
    )

    print("=" * 50)
    print(f"  ✅ Verified:  {result['verified']}")
    print(f"  📏 Distance:  {result['distance']:.4f}")
    print(f"  📊 Threshold: {result['threshold']:.4f}")
    print(f"  🧠 Model:     {result['model']}")
    print(f"  📐 Metric:    {result['similarity_metric']}")
    print("=" * 50)

    if result["verified"]:
        print("\n✅ MATCH — Same person!")
    else:
        print("\n❌ NO MATCH — Different people.")

except ValueError as e:
    print(f"❌ Face detection failed: {e}")
    print("   Make sure both images contain a clearly visible face.")
except Exception as e:
    print(f"❌ Error: {e}")
