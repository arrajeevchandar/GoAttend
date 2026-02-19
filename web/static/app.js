// Attendance Engine Frontend Application
const API_BASE = '';  // Same origin

// State
let accessToken = localStorage.getItem('accessToken');
let deviceId = localStorage.getItem('deviceId');
let tokenExpiry = localStorage.getItem('tokenExpiry');

// Camera state
let cameraStream = null;
let capturedImageData = null;
let currentInputMode = 'camera';

// DOM Elements
const authSection = document.getElementById('authSection');
const dashboard = document.getElementById('dashboard');
const logoutBtn = document.getElementById('logoutBtn');
const connectionStatus = document.getElementById('connectionStatus');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    checkHealth();
    setupEventListeners();
    updateUI();
    
    // Auto-refresh events every 10 seconds
    setInterval(() => {
        if (accessToken) refreshEvents();
    }, 10000);
});

function setupEventListeners() {
    // Register form
    document.getElementById('registerForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await registerDevice();
    });

    // Check-in form
    document.getElementById('checkinForm').addEventListener('submit', async (e) => {
        e.preventDefault();
        await submitCheckin();
    });

    // File upload
    const dropZone = document.getElementById('dropZone');
    const fileInput = document.getElementById('imageFileInput');
    
    if (dropZone) {
        dropZone.addEventListener('click', () => fileInput.click());
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('border-purple-500', 'bg-purple-50');
        });
        dropZone.addEventListener('dragleave', () => {
            dropZone.classList.remove('border-purple-500', 'bg-purple-50');
        });
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.classList.remove('border-purple-500', 'bg-purple-50');
            if (e.dataTransfer.files.length) {
                fileInput.files = e.dataTransfer.files;
                handleFileSelect(e.dataTransfer.files[0]);
            }
        });
    }
    
    if (fileInput) {
        fileInput.addEventListener('change', (e) => {
            if (e.target.files.length) {
                handleFileSelect(e.target.files[0]);
            }
        });
    }
}

// Input mode switching
function setInputMode(mode) {
    currentInputMode = mode;
    
    // Update buttons
    ['camera', 'upload', 'url'].forEach(m => {
        const btn = document.getElementById(`${m}ModeBtn`);
        const section = document.getElementById(`${m}Mode`);
        
        if (btn) {
            if (m === mode) {
                btn.classList.add('bg-white', 'shadow', 'text-purple-600');
                btn.classList.remove('text-gray-600');
            } else {
                btn.classList.remove('bg-white', 'shadow', 'text-purple-600');
                btn.classList.add('text-gray-600');
            }
        }
        
        if (section) {
            section.classList.toggle('hidden', m !== mode);
        }
    });
    
    // Stop camera if switching away
    if (mode !== 'camera' && cameraStream) {
        stopCamera();
    }
}

// Camera functions
async function startCamera() {
    const video = document.getElementById('cameraVideo');
    const startBtn = document.getElementById('startCameraBtn');
    const captureBtn = document.getElementById('captureBtn');
    const overlay = document.getElementById('faceOverlay');
    
    try {
        // Request camera access
        cameraStream = await navigator.mediaDevices.getUserMedia({
            video: {
                width: { ideal: 640 },
                height: { ideal: 480 },
                facingMode: 'user'  // Front camera
            },
            audio: false
        });
        
        video.srcObject = cameraStream;
        await video.play();
        
        startBtn.innerHTML = '<i class="fas fa-stop mr-1"></i> Stop Camera';
        startBtn.onclick = stopCamera;
        startBtn.classList.remove('bg-gray-600');
        startBtn.classList.add('bg-red-600', 'hover:bg-red-700');
        
        captureBtn.disabled = false;
        
        // Hide captured preview, show live feed
        document.getElementById('capturedPreview').classList.add('hidden');
        document.getElementById('cameraContainer').classList.remove('hidden');
        
        showToast('Camera started! Position your face in the oval.', 'success');
        
    } catch (err) {
        console.error('Camera error:', err);
        showToast('Could not access camera. Please check permissions.', 'error');
    }
}

function stopCamera() {
    if (cameraStream) {
        cameraStream.getTracks().forEach(track => track.stop());
        cameraStream = null;
    }
    
    const video = document.getElementById('cameraVideo');
    const startBtn = document.getElementById('startCameraBtn');
    const captureBtn = document.getElementById('captureBtn');
    
    if (video) video.srcObject = null;
    
    if (startBtn) {
        startBtn.innerHTML = '<i class="fas fa-video mr-1"></i> Start Camera';
        startBtn.onclick = startCamera;
        startBtn.classList.remove('bg-red-600', 'hover:bg-red-700');
        startBtn.classList.add('bg-gray-600', 'hover:bg-gray-700');
    }
    
    if (captureBtn) captureBtn.disabled = true;
}

function capturePhoto() {
    const video = document.getElementById('cameraVideo');
    const flash = document.getElementById('captureFlash');
    
    if (!video || !cameraStream) {
        showToast('Please start the camera first.', 'error');
        return;
    }
    
    // Create canvas to capture frame
    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext('2d');
    
    // Flip horizontally to match mirror view
    ctx.translate(canvas.width, 0);
    ctx.scale(-1, 1);
    ctx.drawImage(video, 0, 0);
    
    // Get image data
    capturedImageData = canvas.toDataURL('image/jpeg', 0.9);
    
    // Flash effect
    flash.classList.remove('opacity-0');
    flash.classList.add('capture-flash');
    setTimeout(() => {
        flash.classList.add('opacity-0');
        flash.classList.remove('capture-flash');
    }, 300);
    
    // Show preview
    const capturedImg = document.getElementById('capturedImage');
    capturedImg.src = capturedImageData;
    
    document.getElementById('cameraContainer').classList.add('hidden');
    document.getElementById('capturedPreview').classList.remove('hidden');
    
    // Stop camera to save resources
    stopCamera();
    
    showToast('Photo captured! Review and submit.', 'success');
}

function retakePhoto() {
    capturedImageData = null;
    document.getElementById('capturedPreview').classList.add('hidden');
    document.getElementById('cameraContainer').classList.remove('hidden');
    startCamera();
}

// File handling
function handleFileSelect(file) {
    showFileName(file.name);
    
    // Show preview
    const reader = new FileReader();
    reader.onload = (e) => {
        const preview = document.getElementById('uploadPreview');
        const img = document.getElementById('uploadedImage');
        if (preview && img) {
            img.src = e.target.result;
            preview.classList.remove('hidden');
        }
    };
    reader.readAsDataURL(file);
}

function showFileName(name) {
    const fileNameEl = document.getElementById('fileName');
    fileNameEl.textContent = name;
    fileNameEl.classList.remove('hidden');
}

async function checkHealth() {
    try {
        const res = await fetch(`${API_BASE}/healthz`);
        const data = await res.json();
        
        const statusEl = connectionStatus;
        if (data.status === 'ok' && data.db && data.redis) {
            statusEl.innerHTML = `
                <span class="w-3 h-3 bg-green-400 rounded-full mr-2 animate-pulse"></span>
                <span class="text-sm">Connected</span>
            `;
        } else {
            statusEl.innerHTML = `
                <span class="w-3 h-3 bg-yellow-400 rounded-full mr-2"></span>
                <span class="text-sm">Degraded</span>
            `;
        }
    } catch (err) {
        connectionStatus.innerHTML = `
            <span class="w-3 h-3 bg-red-400 rounded-full mr-2"></span>
            <span class="text-sm">Offline</span>
        `;
    }
}

async function registerDevice() {
    const deviceIdInput = document.getElementById('deviceIdInput');
    const errorEl = document.getElementById('registerError');
    const newDeviceId = deviceIdInput.value.trim();
    
    if (!newDeviceId) {
        showError(errorEl, 'Please enter a device ID');
        return;
    }
    
    try {
        const res = await fetch(`${API_BASE}/v1/devices/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_id: newDeviceId })
        });
        
        const data = await res.json();
        
        if (!res.ok) {
            showError(errorEl, data.error || 'Registration failed');
            return;
        }
        
        // Save credentials
        accessToken = data.access_token;
        deviceId = newDeviceId;
        tokenExpiry = data.expires_at;
        
        localStorage.setItem('accessToken', accessToken);
        localStorage.setItem('deviceId', deviceId);
        localStorage.setItem('tokenExpiry', tokenExpiry);
        localStorage.setItem('refreshToken', data.refresh_token);
        
        showToast('Device registered successfully!', 'success');
        updateUI();
        refreshEvents();
        
    } catch (err) {
        showError(errorEl, 'Network error. Please try again.');
    }
}

async function submitCheckin() {
    const userId = document.getElementById('userIdInput').value.trim();
    const imageUrl = document.getElementById('imageUrlInput').value.trim();
    const fileInput = document.getElementById('imageFileInput');
    const resultEl = document.getElementById('checkinResult');
    const submitBtn = document.getElementById('submitBtn');
    
    if (!userId) {
        showResult(resultEl, 'Please enter an employee ID', 'error');
        return;
    }
    
    let finalImageUrl = '';
    
    // Determine image source based on mode
    if (currentInputMode === 'camera' && capturedImageData) {
        finalImageUrl = capturedImageData;
    } else if (currentInputMode === 'upload' && fileInput.files.length > 0) {
        finalImageUrl = await fileToDataUrl(fileInput.files[0]);
    } else if (currentInputMode === 'url' && imageUrl) {
        finalImageUrl = imageUrl;
    }
    
    if (!finalImageUrl) {
        showResult(resultEl, 'Please capture a photo, upload an image, or enter an image URL', 'error');
        return;
    }
    
    // Show loading state
    submitBtn.disabled = true;
    submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Processing...';
    
    try {
        // Upload local image (base64 / file) to Cloudinary first; skip for plain URLs
        const isLocalImage = finalImageUrl.startsWith('data:');
        if (isLocalImage) {
            submitBtn.innerHTML = '<i class="fas fa-cloud-upload-alt fa-spin mr-2"></i>Uploading image...';
            const uploadRes = await fetch(`${API_BASE}/v1/upload`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${accessToken}`
                },
                body: JSON.stringify({ data: finalImageUrl })
            });
            const uploadData = await uploadRes.json();
            if (!uploadRes.ok) {
                // Fall back to base64 URL if Cloudinary is not configured
                if (uploadRes.status === 503) {
                    console.warn('Cloudinary not configured, falling back to base64');
                } else {
                    showResult(resultEl, uploadData.error || 'Image upload failed', 'error');
                    return;
                }
            } else {
                finalImageUrl = uploadData.url;
            }
            submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Submitting check-in...';
        }

        const res = await fetch(`${API_BASE}/v1/checkins`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${accessToken}`
            },
            body: JSON.stringify({
                user_id: userId,
                device_id: deviceId,
                image_url: finalImageUrl
            })
        });
        
        const data = await res.json();
        
        if (!res.ok) {
            if (res.status === 401) {
                logout();
                showToast('Session expired. Please login again.', 'error');
                return;
            }
            showResult(resultEl, data.error || 'Check-in failed', 'error');
            return;
        }
        
        showResult(resultEl, `✓ Check-in submitted! Event ID: ${data.event_id.slice(0, 8)}...`, 'success');
        showToast('Face check-in submitted for processing!', 'success');
        
        // Show face status panel
        const faceStatus = document.getElementById('faceStatus');
        if (faceStatus) {
            faceStatus.classList.remove('hidden');
            document.getElementById('faceStatusText').textContent = 'Processing...';
            document.getElementById('faceConfidenceBar').style.width = '0%';
        }
        
        // Clear form
        document.getElementById('userIdInput').value = '';
        document.getElementById('imageUrlInput').value = '';
        fileInput.value = '';
        document.getElementById('fileName').classList.add('hidden');
        capturedImageData = null;
        
        // Hide previews
        document.getElementById('capturedPreview')?.classList.add('hidden');
        document.getElementById('uploadPreview')?.classList.add('hidden');
        document.getElementById('cameraContainer')?.classList.remove('hidden');
        
        // Refresh events to see the new one
        setTimeout(refreshEvents, 1000);
        
        // Poll for result
        pollEventStatus(data.event_id);
        
    } catch (err) {
        showResult(resultEl, 'Network error. Please try again.', 'error');
    } finally {
        submitBtn.disabled = false;
        submitBtn.innerHTML = '<i class="fas fa-paper-plane mr-2"></i>Submit Check-in';
    }
}

// Poll event status to show face recognition result
async function pollEventStatus(eventId) {
    const faceStatus = document.getElementById('faceStatus');
    const statusText = document.getElementById('faceStatusText');
    const confidenceBar = document.getElementById('faceConfidenceBar');
    
    let attempts = 0;
    const maxAttempts = 20;
    
    const poll = async () => {
        if (attempts >= maxAttempts) {
            statusText.textContent = 'Timeout - check events table';
            return;
        }
        
        attempts++;
        
        try {
            const res = await fetch(`${API_BASE}/v1/events?limit=50`, {
                headers: { 'Authorization': `Bearer ${accessToken}` }
            });
            
            if (!res.ok) return;
            
            const data = await res.json();
            const event = data.events?.find(e => e.ID === eventId);
            
            if (event) {
                if (event.Status === 'processed') {
                    const score = event.MatchScore || 0;
                    const percent = Math.round(score * 100);
                    
                    statusText.innerHTML = `<span class="text-green-600">✓ Face Detected</span> - ${percent}% confidence`;
                    confidenceBar.style.width = `${percent}%`;
                    confidenceBar.className = `h-full transition-all duration-300 ${percent >= 80 ? 'bg-green-500' : percent >= 50 ? 'bg-yellow-500' : 'bg-red-500'}`;
                    
                    showToast(`Face verified with ${percent}% confidence!`, percent >= 50 ? 'success' : 'error');
                    refreshEvents();
                    return;
                } else if (event.Status === 'failed') {
                    statusText.innerHTML = '<span class="text-red-600">✗ Face Detection Failed</span>';
                    confidenceBar.style.width = '0%';
                    showToast('Face detection failed. Please try again.', 'error');
                    return;
                }
            }
            
            // Still pending, continue polling
            statusText.textContent = `Processing... (${attempts}/${maxAttempts})`;
            confidenceBar.style.width = `${(attempts / maxAttempts) * 30}%`;
            setTimeout(poll, 1500);
            
        } catch (err) {
            console.error('Poll error:', err);
        }
    };
    
    poll();
}

async function refreshEvents() {
    if (!accessToken) return;
    
    try {
        const res = await fetch(`${API_BASE}/v1/events?limit=20`, {
            headers: { 'Authorization': `Bearer ${accessToken}` }
        });
        
        if (res.status === 401) {
            logout();
            return;
        }
        
        const data = await res.json();
        renderEvents(data.events || []);
        updateStats(data.events || []);
        
    } catch (err) {
        console.error('Failed to fetch events:', err);
    }
}

function renderEvents(events) {
    const tbody = document.getElementById('eventsTableBody');
    
    if (!events.length) {
        tbody.innerHTML = `
            <tr>
                <td colspan="4" class="py-8 text-center text-gray-400">
                    <i class="fas fa-inbox text-4xl mb-2 block"></i>
                    <p>No events yet</p>
                </td>
            </tr>
        `;
        return;
    }
    
    tbody.innerHTML = events.map(event => {
        const time = new Date(event.When).toLocaleString();
        const statusClass = getStatusClass(event.Status);
        const score = event.MatchScore ? (event.MatchScore * 100).toFixed(0) + '%' : '--';
        
        return `
            <tr class="border-b border-gray-100 hover:bg-gray-50 slide-in">
                <td class="py-3 px-2">
                    <div class="flex items-center">
                        <div class="w-8 h-8 bg-purple-100 rounded-full flex items-center justify-center mr-2">
                            <i class="fas fa-user text-purple-600 text-xs"></i>
                        </div>
                        <span class="font-medium text-gray-800">${event.UserID}</span>
                    </div>
                </td>
                <td class="py-3 px-2 text-sm text-gray-600">${time}</td>
                <td class="py-3 px-2">
                    <span class="px-2 py-1 rounded-full text-xs font-medium ${statusClass}">
                        ${event.Status || 'pending'}
                    </span>
                </td>
                <td class="py-3 px-2">
                    <span class="text-sm font-medium ${event.MatchScore > 0.8 ? 'text-green-600' : 'text-yellow-600'}">
                        ${score}
                    </span>
                </td>
            </tr>
        `;
    }).join('');
}

function getStatusClass(status) {
    switch (status) {
        case 'processed':
            return 'bg-green-100 text-green-700';
        case 'failed':
            return 'bg-red-100 text-red-700';
        default:
            return 'bg-yellow-100 text-yellow-700';
    }
}

function updateStats(events) {
    const total = events.length;
    const processed = events.filter(e => e.Status === 'processed').length;
    const pending = events.filter(e => e.Status === 'pending' || !e.Status).length;
    
    const scores = events.filter(e => e.MatchScore > 0).map(e => e.MatchScore);
    const avgScore = scores.length > 0 
        ? (scores.reduce((a, b) => a + b, 0) / scores.length * 100).toFixed(0) + '%'
        : '--';
    
    document.getElementById('statTotal').textContent = total;
    document.getElementById('statProcessed').textContent = processed;
    document.getElementById('statPending').textContent = pending;
    document.getElementById('statScore').textContent = avgScore;
}

function updateUI() {
    if (accessToken && deviceId) {
        authSection.classList.add('hidden');
        dashboard.classList.remove('hidden');
        logoutBtn.classList.remove('hidden');
        
        document.getElementById('currentDeviceId').textContent = deviceId;
        
        if (tokenExpiry) {
            const expiryDate = new Date(tokenExpiry * 1000);
            document.getElementById('tokenExpiry').textContent = expiryDate.toLocaleTimeString();
        }
    } else {
        authSection.classList.remove('hidden');
        dashboard.classList.add('hidden');
        logoutBtn.classList.add('hidden');
    }
}

function logout() {
    localStorage.removeItem('accessToken');
    localStorage.removeItem('deviceId');
    localStorage.removeItem('tokenExpiry');
    localStorage.removeItem('refreshToken');
    
    accessToken = null;
    deviceId = null;
    tokenExpiry = null;
    
    updateUI();
    showToast('Logged out successfully', 'info');
}

function showError(el, message) {
    el.textContent = message;
    el.classList.remove('hidden');
    setTimeout(() => el.classList.add('hidden'), 5000);
}

function showResult(el, message, type) {
    el.textContent = message;
    el.classList.remove('hidden', 'bg-green-100', 'text-green-700', 'bg-red-100', 'text-red-700');
    
    if (type === 'success') {
        el.classList.add('bg-green-100', 'text-green-700');
    } else {
        el.classList.add('bg-red-100', 'text-red-700');
    }
    
    setTimeout(() => el.classList.add('hidden'), 5000);
}

function showToast(message, type = 'success') {
    const toast = document.getElementById('toast');
    const icon = document.getElementById('toastIcon');
    const msg = document.getElementById('toastMessage');
    
    msg.textContent = message;
    
    icon.className = 'fas';
    switch (type) {
        case 'success':
            icon.classList.add('fa-check-circle', 'text-green-400');
            break;
        case 'error':
            icon.classList.add('fa-exclamation-circle', 'text-red-400');
            break;
        default:
            icon.classList.add('fa-info-circle', 'text-blue-400');
    }
    
    toast.classList.remove('hidden');
    setTimeout(() => toast.classList.add('hidden'), 3000);
}

function fileToDataUrl(file) {
    return new Promise((resolve) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result);
        reader.readAsDataURL(file);
    });
}

// Expose functions globally
window.logout = logout;
window.refreshEvents = refreshEvents;
window.setInputMode = setInputMode;
window.startCamera = startCamera;
window.stopCamera = stopCamera;
window.capturePhoto = capturePhoto;
window.retakePhoto = retakePhoto;
