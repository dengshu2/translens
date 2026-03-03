// ── State ────────────────────────────────────────────
let isTranslating = false;
const PAGE_SIZE = 20;
let historyOffset = 0;
let historyTotal = 0;
let searchQuery = '';
let searchDebounceTimer = null;

// ── DOM ──────────────────────────────────────────────
const inputEl = document.getElementById('input-chinese');
const charCountEl = document.getElementById('char-count');
const textareaWrapper = document.getElementById('textarea-wrapper');
const btnTranslate = document.getElementById('btn-translate');
const resultSection = document.getElementById('result-section');
const resultEnglish = document.getElementById('result-english');
const resultChinese = document.getElementById('result-chinese');
const resultCopyBtn = document.getElementById('result-copy-btn');
const historyEmpty = document.getElementById('history-empty');
const searchEmpty = document.getElementById('search-empty');
const historyError = document.getElementById('history-error');
const skeletonList = document.getElementById('skeleton-list');
const historyList = document.getElementById('history-list');
const historyCountEl = document.getElementById('history-count');
const loadMoreWrapper = document.getElementById('load-more-wrapper');
const loadMoreBtn = document.getElementById('btn-load-more');
const toastContainer = document.getElementById('toast-container');
const searchInputEl = document.getElementById('search-input');
const searchClearBtn = document.getElementById('search-clear');
const btnTheme = document.getElementById('btn-theme');
const themeIcon = document.getElementById('theme-icon');

// ── Theme ────────────────────────────────────────────
function getEffectiveTheme() {
    const saved = document.documentElement.dataset.theme;
    if (saved) return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light';
}

function updateThemeIcon() {
    if (themeIcon) {
        themeIcon.textContent = getEffectiveTheme() === 'dark' ? '☀️' : '🌙';
    }
}

function initTheme() {
    const saved = localStorage.getItem('translens-theme');
    if (saved) {
        document.documentElement.dataset.theme = saved;
    }
    updateThemeIcon();
}

function toggleTheme() {
    const next = getEffectiveTheme() === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('translens-theme', next);
    updateThemeIcon();
}

if (btnTheme) {
    btnTheme.addEventListener('click', toggleTheme);
}

window
    .matchMedia('(prefers-color-scheme: dark)')
    .addEventListener('change', updateThemeIcon);

initTheme();

// ── Char Count ───────────────────────────────────────
inputEl.addEventListener('input', () => {
    const len = [...inputEl.value].length;
    charCountEl.textContent = `${len}/500`;
    charCountEl.classList.toggle('warning', len > 450);
    btnTranslate.disabled = inputEl.value.trim().length === 0;
});

// ── Keyboard Shortcut ────────────────────────────────
inputEl.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        if (!isTranslating && inputEl.value.trim()) {
            doTranslate();
        }
    }
});

// ── Translate Button ─────────────────────────────────
btnTranslate.addEventListener('click', () => {
    if (!isTranslating && inputEl.value.trim()) {
        doTranslate();
    }
});

// ── Copy Result ──────────────────────────────────────
resultCopyBtn.addEventListener('click', () => {
    copyToClipboard(resultEnglish.textContent, resultCopyBtn);
});

// ── Translate Logic ──────────────────────────────────
async function doTranslate(chinese) {
    const text = chinese || inputEl.value.trim();
    if (!text || isTranslating) return;

    setLoading(true);

    try {
        const res = await fetch('/api/translate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ chinese: text }),
        });

        const data = await res.json();

        if (!res.ok) {
            throw new Error(data.error || '翻译失败');
        }

        resultEnglish.textContent = data.english;
        resultChinese.textContent = data.chinese;
        resultSection.style.display = '';
        resultCopyBtn.classList.remove('copied');
        resultCopyBtn.textContent = '📋';

        addHistoryItem(data, true);
        historyEmpty.style.display = 'none';
        if (searchEmpty) searchEmpty.style.display = 'none';
        historyOffset++;
        historyTotal++;
        updateHistoryCount();

        if (!chinese) {
            inputEl.value = '';
            charCountEl.textContent = '0/500';
            btnTranslate.disabled = true;
        }
    } catch (err) {
        showToast(err.message, 'error');
    } finally {
        setLoading(false);
    }
}

function setLoading(loading) {
    isTranslating = loading;
    btnTranslate.disabled = loading;
    textareaWrapper.classList.toggle('disabled', loading);

    if (loading) {
        btnTranslate.innerHTML =
            '<span class="spinner"></span><span class="btn-text">翻译中...</span>';
    } else {
        btnTranslate.innerHTML = '<span class="btn-text">翻译</span>';
    }
}

// ── History ──────────────────────────────────────────
function addHistoryItem(t, prepend) {
    const li = document.createElement('li');
    li.className = 'history-item';
    li.dataset.id = t.id;

    const timeStr = timeAgo(t.created_at);

    li.innerHTML = `
        <div class="history-item-chinese">${escapeHtml(t.chinese)}</div>
        <div class="history-item-english">${escapeHtml(t.english)}</div>
        <div class="history-item-meta">
            <time class="history-item-time">${escapeHtml(timeStr)}</time>
            <div class="history-item-actions">
                <button class="btn-icon" data-action="retranslate" aria-label="重新翻译">🔄</button>
                <button class="btn-icon" data-action="copy" aria-label="复制翻译结果">📋</button>
                <button class="btn-icon btn-delete" data-action="delete" aria-label="删除记录">🗑️</button>
            </div>
        </div>
    `;

    // Set data via DOM API — safe from XSS
    const timeEl = li.querySelector('.history-item-time');
    timeEl.title = new Date(t.created_at).toLocaleString();
    timeEl.setAttribute('datetime', t.created_at);

    li.querySelector('[data-action="retranslate"]').dataset.text = t.chinese;
    li.querySelector('[data-action="copy"]').dataset.text = t.english;
    li.querySelector('[data-action="delete"]').dataset.deleteId = String(t.id);

    if (prepend) {
        historyList.prepend(li);
    } else {
        historyList.appendChild(li);
    }
}

// ── Event Delegation for History Actions ─────────────
historyList.addEventListener('click', (e) => {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;

    const action = btn.dataset.action;

    if (action === 'retranslate') {
        doTranslate(btn.dataset.text);
    } else if (action === 'copy') {
        copyToClipboard(btn.dataset.text, btn);
    } else if (action === 'delete') {
        deleteTranslation(Number(btn.dataset.deleteId), btn);
    }
});

async function deleteTranslation(id, btnEl) {
    if (!confirm('确定要删除这条记录吗？')) return;

    try {
        const res = await fetch(`/api/translations/${id}`, { method: 'DELETE' });
        if (!res.ok) {
            const data = await res.json();
            throw new Error(data.error || '删除失败');
        }

        const li = btnEl.closest('.history-item');
        li.classList.add('removing');
        setTimeout(() => {
            li.remove();
            historyOffset--;
            historyTotal--;
            updateHistoryCount();
            if (historyTotal <= 0) {
                historyEmpty.style.display = '';
            }
        }, 300);

        showToast('已删除', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function updateHistoryCount() {
    if (historyTotal > 0) {
        historyCountEl.textContent = `(${Math.min(historyOffset, historyTotal)}/${historyTotal})`;
    } else {
        historyCountEl.textContent = '';
    }
}

// ── Skeleton ─────────────────────────────────────────
function showSkeleton() {
    skeletonList.style.display = '';
}

function hideSkeleton() {
    skeletonList.style.display = 'none';
}

// ── Load History ─────────────────────────────────────
async function loadHistory(append) {
    if (loadMoreBtn) loadMoreBtn.disabled = true;
    historyError.style.display = 'none';

    if (!append) showSkeleton();

    try {
        const searchParam = searchQuery
            ? `&q=${encodeURIComponent(searchQuery)}`
            : '';
        const res = await fetch(
            `/api/history?limit=${PAGE_SIZE}&offset=${historyOffset}${searchParam}`
        );
        const data = await res.json();

        historyTotal = data.total || 0;

        hideSkeleton();

        if (data.translations && data.translations.length > 0) {
            historyEmpty.style.display = 'none';
            if (searchEmpty) searchEmpty.style.display = 'none';
            data.translations.forEach((t) => addHistoryItem(t));
            historyOffset += data.translations.length;
        } else if (!append && historyTotal === 0) {
            if (searchQuery) {
                historyEmpty.style.display = 'none';
                if (searchEmpty) searchEmpty.style.display = '';
            } else {
                historyEmpty.style.display = '';
                if (searchEmpty) searchEmpty.style.display = 'none';
            }
        }

        if (data.has_more) {
            loadMoreWrapper.style.display = '';
        } else {
            loadMoreWrapper.style.display = 'none';
        }

        updateHistoryCount();
    } catch (err) {
        console.error('Failed to load history:', err);
        hideSkeleton();
        historyEmpty.style.display = 'none';
        if (searchEmpty) searchEmpty.style.display = 'none';
        historyError.style.display = 'flex';
    } finally {
        if (loadMoreBtn) loadMoreBtn.disabled = false;
    }
}

loadMoreBtn.addEventListener('click', () => {
    loadHistory(true);
});

// ── Retry ────────────────────────────────────────────
document.getElementById('btn-retry').addEventListener('click', () => {
    historyOffset = 0;
    historyList.innerHTML = '';
    loadHistory();
});

// ── Search ───────────────────────────────────────────
searchInputEl.addEventListener('input', () => {
    const val = searchInputEl.value.trim();
    searchClearBtn.style.display = val ? 'flex' : 'none';

    clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
        searchQuery = val;
        historyOffset = 0;
        historyList.innerHTML = '';
        loadHistory();
    }, 300);
});

searchClearBtn.addEventListener('click', () => {
    searchInputEl.value = '';
    searchClearBtn.style.display = 'none';
    searchQuery = '';
    historyOffset = 0;
    historyList.innerHTML = '';
    loadHistory();
});

// ── Clipboard ────────────────────────────────────────
async function copyToClipboard(text, btnEl) {
    try {
        await navigator.clipboard.writeText(text);
        const original = btnEl.textContent;
        btnEl.textContent = '✓';
        btnEl.classList.add('copied');
        setTimeout(() => {
            btnEl.textContent = original;
            btnEl.classList.remove('copied');
        }, 1500);
    } catch (err) {
        showToast('复制失败', 'error');
    }
}

// ── Toast ────────────────────────────────────────────
function showToast(message, type) {
    const toast = document.createElement('div');
    toast.className = `toast ${type || 'error'}`;
    toast.textContent = message;
    toastContainer.appendChild(toast);

    setTimeout(() => {
        toast.remove();
    }, 3000);
}

// ── Time Helper ──────────────────────────────────────
function timeAgo(dateStr) {
    const now = new Date();
    const date = new Date(dateStr);
    const seconds = Math.floor((now - date) / 1000);

    if (seconds < 60) return '刚刚';
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes} 分钟前`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} 小时前`;
    const days = Math.floor(hours / 24);
    if (days < 30) return `${days} 天前`;
    const months = Math.floor(days / 30);
    if (months < 12) return `${months} 个月前`;
    return `${Math.floor(months / 12)} 年前`;
}

// ── Escape Helper ────────────────────────────────────
function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// ── Init ─────────────────────────────────────────────
loadHistory();
