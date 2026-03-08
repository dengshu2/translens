// ════════════════════════════════════════════════════
// Theme
// ════════════════════════════════════════════════════
const btnTheme = document.getElementById('btn-theme');
const themeIcon = document.getElementById('theme-icon');

function getEffectiveTheme() {
    const saved = document.documentElement.dataset.theme;
    if (saved) return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function updateThemeIcon() {
    if (themeIcon) themeIcon.textContent = getEffectiveTheme() === 'dark' ? '☀️' : '🌙';
}

function initTheme() {
    const saved = localStorage.getItem('translens-theme');
    if (saved) document.documentElement.dataset.theme = saved;
    updateThemeIcon();
}

function toggleTheme() {
    const next = getEffectiveTheme() === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('translens-theme', next);
    updateThemeIcon();
}

if (btnTheme) btnTheme.addEventListener('click', toggleTheme);
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', updateThemeIcon);
initTheme();

// ════════════════════════════════════════════════════
// Tab Switch
// ════════════════════════════════════════════════════
const tabTranslate = document.getElementById('tab-translate');
const tabCorrect = document.getElementById('tab-correct');
const panelTranslate = document.getElementById('panel-translate');
const panelCorrect = document.getElementById('panel-correct');

let currentMode = 'translate';

function switchMode(mode) {
    currentMode = mode;
    const isTranslate = mode === 'translate';

    tabTranslate.classList.toggle('active', isTranslate);
    tabCorrect.classList.toggle('active', !isTranslate);
    tabTranslate.setAttribute('aria-selected', String(isTranslate));
    tabCorrect.setAttribute('aria-selected', String(!isTranslate));

    panelTranslate.style.display = isTranslate ? '' : 'none';
    panelCorrect.style.display = isTranslate ? 'none' : '';

    // Lazy-load correction history on first visit
    if (!isTranslate && correctHistoryOffset === 0 && !correctHistoryLoaded) {
        loadCorrectionHistory();
    }
}

tabTranslate.addEventListener('click', () => switchMode('translate'));
tabCorrect.addEventListener('click', () => switchMode('correct'));

// ════════════════════════════════════════════════════
// Utilities
// ════════════════════════════════════════════════════
const toastContainer = document.getElementById('toast-container');

function showToast(message, type) {
    const toast = document.createElement('div');
    toast.className = `toast ${type || 'error'}`;
    toast.textContent = message;
    toastContainer.appendChild(toast);
    setTimeout(() => toast.remove(), 3000);
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function timeAgo(dateStr) {
    const seconds = Math.floor((Date.now() - new Date(dateStr)) / 1000);
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
    } catch {
        showToast('复制失败', 'error');
    }
}

// ════════════════════════════════════════════════════
// Word-level diff helper
// ════════════════════════════════════════════════════
/**
 * Simple LCS-based word diff.
 * Returns an HTML string with <span class="diff-del"> and <span class="diff-ins"> markers.
 * If original === corrected, returns a "no changes" notice instead.
 */
function buildDiffHtml(original, corrected) {
    if (original === corrected) {
        return `<span class="correct-unchanged">✓ 未发现语法错误，无需修改</span>`;
    }

    const aWords = tokenize(original);
    const bWords = tokenize(corrected);
    const lcs = computeLCS(aWords, bWords);

    let ai = 0, bi = 0, li = 0;
    const parts = [];

    while (ai < aWords.length || bi < bWords.length) {
        if (li < lcs.length && ai < aWords.length && bi < bWords.length &&
            aWords[ai] === lcs[li] && bWords[bi] === lcs[li]) {
            // Common token — output as-is
            parts.push(escapeHtml(aWords[ai]));
            ai++; bi++; li++;
        } else {
            // Collect consecutive deletions and insertions
            let dels = [];
            let ins = [];

            while (ai < aWords.length && (li >= lcs.length || aWords[ai] !== lcs[li])) {
                dels.push(aWords[ai++]);
            }
            while (bi < bWords.length && (li >= lcs.length || bWords[bi] !== lcs[li])) {
                ins.push(bWords[bi++]);
            }

            if (dels.length > 0) {
                parts.push(`<span class="diff-del">${escapeHtml(dels.join(''))}</span>`);
            }
            if (ins.length > 0) {
                parts.push(`<span class="diff-ins">${escapeHtml(ins.join(''))}</span>`);
            }
        }
    }

    return parts.join('');
}

/** Tokenize into words + whitespace tokens so spacing is preserved. */
function tokenize(text) {
    return text.match(/\S+|\s+/g) || [];
}

/** Standard LCS on word-token arrays. */
function computeLCS(a, b) {
    const m = a.length, n = b.length;
    const dp = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
    for (let i = 1; i <= m; i++) {
        for (let j = 1; j <= n; j++) {
            dp[i][j] = a[i - 1] === b[j - 1]
                ? dp[i - 1][j - 1] + 1
                : Math.max(dp[i - 1][j], dp[i][j - 1]);
        }
    }
    // Backtrack
    const result = [];
    let i = m, j = n;
    while (i > 0 && j > 0) {
        if (a[i - 1] === b[j - 1]) { result.push(a[i - 1]); i--; j--; }
        else if (dp[i - 1][j] > dp[i][j - 1]) { i--; }
        else { j--; }
    }
    return result.reverse();
}

// ════════════════════════════════════════════════════
// ── TRANSLATION ──────────────────────────────────────
// ════════════════════════════════════════════════════
let isTranslating = false;
const PAGE_SIZE = 20;
let historyOffset = 0;
let historyTotal = 0;
let searchQuery = '';
let searchDebounceTimer = null;

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
const searchInputEl = document.getElementById('search-input');
const searchClearBtn = document.getElementById('search-clear');

// Char count
inputEl.addEventListener('input', () => {
    const len = [...inputEl.value].length;
    charCountEl.textContent = `${len}/500`;
    charCountEl.classList.toggle('warning', len > 450);
    btnTranslate.disabled = inputEl.value.trim().length === 0;
});

// Keyboard shortcut
inputEl.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        if (!isTranslating && inputEl.value.trim()) doTranslate();
    }
});

btnTranslate.addEventListener('click', () => {
    if (!isTranslating && inputEl.value.trim()) doTranslate();
});

resultCopyBtn.addEventListener('click', () => {
    copyToClipboard(resultEnglish.textContent, resultCopyBtn);
});

async function doTranslate(chinese) {
    const text = chinese || inputEl.value.trim();
    if (!text || isTranslating) return;

    setTranslateLoading(true);

    try {
        const res = await fetch('/api/translate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ chinese: text }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || '翻译失败');

        resultEnglish.textContent = data.english;
        resultChinese.textContent = data.chinese;
        resultSection.style.display = '';
        resultCopyBtn.classList.remove('copied');
        resultCopyBtn.textContent = '📋';

        addTranslationItem(data, true);
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
        setTranslateLoading(false);
    }
}

function setTranslateLoading(loading) {
    isTranslating = loading;
    btnTranslate.disabled = loading;
    textareaWrapper.classList.toggle('disabled', loading);
    btnTranslate.innerHTML = loading
        ? '<span class="spinner"></span><span class="btn-text">翻译中...</span>'
        : '<span class="btn-text">翻译</span>';
}

function addTranslationItem(t, prepend) {
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

    const timeEl = li.querySelector('.history-item-time');
    timeEl.title = new Date(t.created_at).toLocaleString();
    timeEl.setAttribute('datetime', t.created_at);
    li.querySelector('[data-action="retranslate"]').dataset.text = t.chinese;
    li.querySelector('[data-action="copy"]').dataset.text = t.english;
    li.querySelector('[data-action="delete"]').dataset.deleteId = String(t.id);

    if (prepend) historyList.prepend(li);
    else historyList.appendChild(li);
}

historyList.addEventListener('click', (e) => {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;
    const action = btn.dataset.action;
    if (action === 'retranslate') doTranslate(btn.dataset.text);
    else if (action === 'copy') copyToClipboard(btn.dataset.text, btn);
    else if (action === 'delete') deleteTranslation(Number(btn.dataset.deleteId), btn);
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
            if (historyTotal <= 0) historyEmpty.style.display = '';
        }, 300);
        showToast('已删除', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function updateHistoryCount() {
    historyCountEl.textContent = historyTotal > 0
        ? `(${Math.min(historyOffset, historyTotal)}/${historyTotal})`
        : '';
}

function showSkeleton() { skeletonList.style.display = ''; }
function hideSkeleton() { skeletonList.style.display = 'none'; }

async function loadHistory(append) {
    if (loadMoreBtn) loadMoreBtn.disabled = true;
    historyError.style.display = 'none';
    if (!append) showSkeleton();

    try {
        const q = searchQuery ? `&q=${encodeURIComponent(searchQuery)}` : '';
        const res = await fetch(`/api/history?limit=${PAGE_SIZE}&offset=${historyOffset}${q}`);
        const data = await res.json();
        historyTotal = data.total || 0;
        hideSkeleton();

        if (data.translations && data.translations.length > 0) {
            historyEmpty.style.display = 'none';
            if (searchEmpty) searchEmpty.style.display = 'none';
            data.translations.forEach((t) => addTranslationItem(t));
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

        loadMoreWrapper.style.display = data.has_more ? '' : 'none';
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

loadMoreBtn.addEventListener('click', () => loadHistory(true));

document.getElementById('btn-retry').addEventListener('click', () => {
    historyOffset = 0;
    historyList.innerHTML = '';
    loadHistory();
});

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

// ════════════════════════════════════════════════════
// ── CORRECTION ───────────────────────════════════════
// ════════════════════════════════════════════════════
let isCorrecting = false;
let correctHistoryOffset = 0;
let correctHistoryTotal = 0;
let correctSearch = '';
let correctSearchDebounce = null;
let correctHistoryLoaded = false;

const correctInput = document.getElementById('input-english');
const correctCharCount = document.getElementById('correct-char-count');
const correctWrapper = document.getElementById('correct-textarea-wrapper');
const btnCorrect = document.getElementById('btn-correct');
const correctResultSection = document.getElementById('correct-result-section');
const correctDiff = document.getElementById('correct-diff');
const correctOriginalText = document.getElementById('correct-original-text');
const correctCopyBtn = document.getElementById('correct-copy-btn');
const correctHistoryEmpty = document.getElementById('correct-history-empty');
const correctSearchEmpty = document.getElementById('correct-search-empty');
const correctHistoryError = document.getElementById('correct-history-error');
const correctSkeletonList = document.getElementById('correct-skeleton-list');
const correctHistoryList = document.getElementById('correct-history-list');
const correctHistoryCountEl = document.getElementById('correct-history-count');
const correctLoadMoreWrapper = document.getElementById('correct-load-more-wrapper');
const correctLoadMoreBtn = document.getElementById('correct-btn-load-more');
const correctSearchInput = document.getElementById('correct-search-input');
const correctSearchClear = document.getElementById('correct-search-clear');

// store last corrected text for copy button
let lastCorrected = '';

// Char count
correctInput.addEventListener('input', () => {
    const len = [...correctInput.value].length;
    correctCharCount.textContent = `${len}/1000`;
    correctCharCount.classList.toggle('warning', len > 900);
    btnCorrect.disabled = correctInput.value.trim().length === 0;
});

// Keyboard shortcut
correctInput.addEventListener('keydown', (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        e.preventDefault();
        if (!isCorrecting && correctInput.value.trim()) doCorrect();
    }
});

btnCorrect.addEventListener('click', () => {
    if (!isCorrecting && correctInput.value.trim()) doCorrect();
});

correctCopyBtn.addEventListener('click', () => {
    copyToClipboard(lastCorrected, correctCopyBtn);
});

async function doCorrect() {
    const text = correctInput.value.trim();
    if (!text || isCorrecting) return;

    setCorrectLoading(true);

    try {
        const res = await fetch('/api/correct', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ english: text }),
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || '纠错失败');

        lastCorrected = data.corrected;
        correctDiff.innerHTML = buildDiffHtml(data.original, data.corrected);
        correctOriginalText.textContent = data.original;
        correctResultSection.style.display = '';
        correctCopyBtn.classList.remove('copied');
        correctCopyBtn.textContent = '📋';

        addCorrectionItem(data, true);
        correctHistoryEmpty.style.display = 'none';
        if (correctSearchEmpty) correctSearchEmpty.style.display = 'none';
        correctHistoryOffset++;
        correctHistoryTotal++;
        updateCorrectionCount();

        correctInput.value = '';
        correctCharCount.textContent = '0/1000';
        btnCorrect.disabled = true;
    } catch (err) {
        showToast(err.message, 'error');
    } finally {
        setCorrectLoading(false);
    }
}

function setCorrectLoading(loading) {
    isCorrecting = loading;
    btnCorrect.disabled = loading;
    correctWrapper.classList.toggle('disabled', loading);
    btnCorrect.innerHTML = loading
        ? '<span class="spinner"></span><span class="btn-text">纠错中...</span>'
        : '<span class="btn-text">纠错</span>';
}

function addCorrectionItem(c, prepend) {
    const li = document.createElement('li');
    li.className = 'history-item';
    li.dataset.id = c.id;
    const timeStr = timeAgo(c.created_at);

    li.innerHTML = `
        <div class="history-item-original">${escapeHtml(c.original)}</div>
        <div class="history-item-corrected">${escapeHtml(c.corrected)}</div>
        <div class="history-item-meta">
            <time class="history-item-time">${escapeHtml(timeStr)}</time>
            <div class="history-item-actions">
                <button class="btn-icon" data-action="re-correct" aria-label="重新纠错">🔄</button>
                <button class="btn-icon" data-action="copy" aria-label="复制纠错结果">📋</button>
                <button class="btn-icon btn-delete" data-action="delete" aria-label="删除记录">🗑️</button>
            </div>
        </div>
    `;

    const timeEl = li.querySelector('.history-item-time');
    timeEl.title = new Date(c.created_at).toLocaleString();
    timeEl.setAttribute('datetime', c.created_at);
    li.querySelector('[data-action="re-correct"]').dataset.text = c.original;
    li.querySelector('[data-action="copy"]').dataset.text = c.corrected;
    li.querySelector('[data-action="delete"]').dataset.deleteId = String(c.id);

    if (prepend) correctHistoryList.prepend(li);
    else correctHistoryList.appendChild(li);
}

correctHistoryList.addEventListener('click', (e) => {
    const btn = e.target.closest('[data-action]');
    if (!btn) return;
    const action = btn.dataset.action;
    if (action === 're-correct') {
        correctInput.value = btn.dataset.text;
        correctInput.dispatchEvent(new Event('input'));
        correctInput.focus();
        window.scrollTo({ top: 0, behavior: 'smooth' });
    } else if (action === 'copy') {
        copyToClipboard(btn.dataset.text, btn);
    } else if (action === 'delete') {
        deleteCorrection(Number(btn.dataset.deleteId), btn);
    }
});

async function deleteCorrection(id, btnEl) {
    if (!confirm('确定要删除这条记录吗？')) return;
    try {
        const res = await fetch(`/api/corrections/${id}`, { method: 'DELETE' });
        if (!res.ok) {
            const data = await res.json();
            throw new Error(data.error || '删除失败');
        }
        const li = btnEl.closest('.history-item');
        li.classList.add('removing');
        setTimeout(() => {
            li.remove();
            correctHistoryOffset--;
            correctHistoryTotal--;
            updateCorrectionCount();
            if (correctHistoryTotal <= 0) correctHistoryEmpty.style.display = '';
        }, 300);
        showToast('已删除', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function updateCorrectionCount() {
    correctHistoryCountEl.textContent = correctHistoryTotal > 0
        ? `(${Math.min(correctHistoryOffset, correctHistoryTotal)}/${correctHistoryTotal})`
        : '';
}

function showCorrectSkeleton() { correctSkeletonList.style.display = ''; }
function hideCorrectSkeleton() { correctSkeletonList.style.display = 'none'; }

async function loadCorrectionHistory(append) {
    if (correctLoadMoreBtn) correctLoadMoreBtn.disabled = true;
    correctHistoryError.style.display = 'none';
    if (!append) showCorrectSkeleton();

    try {
        const q = correctSearch ? `&q=${encodeURIComponent(correctSearch)}` : '';
        const res = await fetch(`/api/corrections?limit=${PAGE_SIZE}&offset=${correctHistoryOffset}${q}`);
        const data = await res.json();
        correctHistoryTotal = data.total || 0;
        hideCorrectSkeleton();
        correctHistoryLoaded = true;

        if (data.corrections && data.corrections.length > 0) {
            correctHistoryEmpty.style.display = 'none';
            if (correctSearchEmpty) correctSearchEmpty.style.display = 'none';
            data.corrections.forEach((c) => addCorrectionItem(c));
            correctHistoryOffset += data.corrections.length;
        } else if (!append && correctHistoryTotal === 0) {
            if (correctSearch) {
                correctHistoryEmpty.style.display = 'none';
                if (correctSearchEmpty) correctSearchEmpty.style.display = '';
            } else {
                correctHistoryEmpty.style.display = '';
                if (correctSearchEmpty) correctSearchEmpty.style.display = 'none';
            }
        }

        correctLoadMoreWrapper.style.display = data.has_more ? '' : 'none';
        updateCorrectionCount();
    } catch (err) {
        console.error('Failed to load correction history:', err);
        hideCorrectSkeleton();
        correctHistoryEmpty.style.display = 'none';
        if (correctSearchEmpty) correctSearchEmpty.style.display = 'none';
        correctHistoryError.style.display = 'flex';
    } finally {
        if (correctLoadMoreBtn) correctLoadMoreBtn.disabled = false;
    }
}

correctLoadMoreBtn.addEventListener('click', () => loadCorrectionHistory(true));

document.getElementById('correct-btn-retry').addEventListener('click', () => {
    correctHistoryOffset = 0;
    correctHistoryList.innerHTML = '';
    loadCorrectionHistory();
});

correctSearchInput.addEventListener('input', () => {
    const val = correctSearchInput.value.trim();
    correctSearchClear.style.display = val ? 'flex' : 'none';
    clearTimeout(correctSearchDebounce);
    correctSearchDebounce = setTimeout(() => {
        correctSearch = val;
        correctHistoryOffset = 0;
        correctHistoryList.innerHTML = '';
        loadCorrectionHistory();
    }, 300);
});

correctSearchClear.addEventListener('click', () => {
    correctSearchInput.value = '';
    correctSearchClear.style.display = 'none';
    correctSearch = '';
    correctHistoryOffset = 0;
    correctHistoryList.innerHTML = '';
    loadCorrectionHistory();
});

// ════════════════════════════════════════════════════
// Init
// ════════════════════════════════════════════════════
loadHistory();
