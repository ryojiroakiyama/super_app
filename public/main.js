const API = location.origin;
const listEl = document.getElementById('mailList');
const player = document.getElementById('player');
const filterFrom = document.getElementById('filterFrom');
const filterTitle = document.getElementById('filterTitle');
const filterForm = document.getElementById('filterForm');

// ===== DEBUG =====
console.log('[main.js] loaded at', new Date().toISOString());

// ===== default settings =====
// 初期表示で「週刊Life is beautiful」を検索
filterTitle.value = '週刊Life is beautiful';
// li の • を消すために list-style を none に
listEl.classList.add('list-none');
listEl.style.listStyleType = 'none';

function buildQuery(from, title) {
  const parts = [];
  if (from) parts.push(`from:${from}`);
  if (title) {
    title.trim().split(/\s+/).forEach(t => {
      if (t) parts.push(`subject:${t}`);
    });
  }
  return encodeURIComponent(parts.join(' '));
}

function fetchList() {
  console.log('[fetchList] called');
  const q = buildQuery(filterFrom.value.trim(), filterTitle.value.trim());
  const url = q ? `${API}/messages?max=5&q=${q}` : `${API}/messages?max=5`;
  console.log('[fetchList] url', url);
  fetch(url)
    .then(r => r.json())
    .then(d => {
      console.log('[fetchList] received', d.messages?.length, 'messages');
      listEl.innerHTML = '';
      // Stop current playback when list is refreshed
      player.pause();
      player.removeAttribute('src');
      player.load();
      d.messages.forEach(m => {
        const li = document.createElement('li');
        li.className = 'border p-3 rounded shadow hover:bg-gray-50 mb-2';

        // info block
        const info = document.createElement('div');
        const date = new Date(Number(m.internalDate)).toLocaleString();
        info.innerHTML = `\n          <div class=\"text-sm text-gray-600\">${date}</div>\n          <div class=\"text-sm text-gray-600\">${m.from}</div>\n          <div class=\"font-medium break-words\">${m.subject}</div>\n          <div class=\"text-xs text-gray-500\">${m.preview}</div>`;

        // buttons container
        const btnWrap = document.createElement('div');
        btnWrap.className = 'mt-2 flex gap-2';

        // Stream button
        const streamBtn = document.createElement('button');
        streamBtn.textContent = '▶︎ ストリーム';
        streamBtn.className = 'px-2 py-1 bg-blue-600 text-white text-xs rounded hover:bg-blue-700';
        streamBtn.onclick = () => {
          player.src = `${API}/messages/${m.id}/tts/stream`;
          player.play();
        };

        // Download button
        const dlBtn = document.createElement('button');
        dlBtn.textContent = '⬇︎ ダウンロード';
        dlBtn.className = 'px-2 py-1 bg-green-600 text-white text-xs rounded hover:bg-green-700';
        dlBtn.onclick = async () => {
          try {
            dlBtn.disabled = true;
            dlBtn.textContent = 'ダウンロード中…';
            console.log('[download] generating audio for', m.id);
            const genResp = await fetch(`${API}/messages/${m.id}/tts`, {method: 'POST'});
            if (!genResp.ok) throw new Error('failed to generate audio');
            console.log('[download] generated, downloading file');
            const url = `${API}/audios/merged/${m.id}.mp3`;
            const a = document.createElement('a');
            a.href = url;
            a.download = `${m.id}.mp3`;
            document.body.appendChild(a);
            a.click();
            a.remove();
            console.log('[download] download triggered', url);
          } catch (e) {
            console.error(e);
            alert('ダウンロードに失敗しました');
          } finally {
            dlBtn.disabled = false;
            dlBtn.textContent = '⬇︎ ダウンロード';
          }
        };

        btnWrap.appendChild(streamBtn);
        btnWrap.appendChild(dlBtn);

        li.appendChild(info);
        li.appendChild(btnWrap);
        listEl.appendChild(li);

        console.debug('[fetchList] added mail', m.id);
      });
    })
    .catch(err => console.error(err));
}

filterForm.addEventListener('submit', (e) => {
  e.preventDefault();
  fetchList();
});

document.addEventListener('DOMContentLoaded', fetchList);

// Fallback: fetch list after 500ms in case DOMContentLoaded fired before listener attached
setTimeout(() => {
  if (!listEl.firstChild) {
    console.log('[init] list empty after 500ms, triggering fetchList manually');
    fetchList();
  }
}, 500); 