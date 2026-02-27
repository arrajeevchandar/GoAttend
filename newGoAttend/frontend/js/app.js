const API = '/api';

async function apiGet(path) {
    const res = await fetch(`${API}${path}`);
    if (!res.ok) throw new Error((await res.json()).error || res.statusText);
    return res.json();
}

async function apiPostForm(path, formData) {
    const res = await fetch(`${API}${path}`, { method: 'POST', body: formData });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
}

// Toast notifications
function showToast(message, type = 'success') {
    document.querySelectorAll('.toast').forEach(t => t.remove());
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);
    requestAnimationFrame(() => toast.classList.add('show'));
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// Active nav link
document.addEventListener('DOMContentLoaded', () => {
    const path = window.location.pathname;
    document.querySelectorAll('.navbar nav a').forEach(a => {
        if (a.getAttribute('href') === path) a.classList.add('active');
    });
});

// Date formatter
function formatDate(dateStr) {
    return new Date(dateStr).toLocaleDateString('en-IN', {
        day: '2-digit', month: 'short', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
    });
}
