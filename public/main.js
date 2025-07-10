const API = location.origin;
const listEl = document.getElementById('mailList');
const player = document.getElementById('player');
const filterFrom = document.getElementById('filterFrom');
const filterTitle = document.getElementById('filterTitle');
const filterForm = document.getElementById('filterForm');

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
  const q = buildQuery(filterFrom.value.trim(), filterTitle.value.trim());
  const url = q ? `${API}/messages?max=20&q=${q}` : `${API}/messages?max=20`;
  fetch(url)
    .then(r => r.json())
    .then(d => {
      listEl.innerHTML = '';
      // Stop current playback when list is refreshed
      player.pause();
      player.removeAttribute('src');
      player.load();
      d.messages.forEach(m => {
        const btn = document.createElement('button');
        btn.className = 'w-full text-left border p-3 rounded shadow hover:bg-gray-50';
        const date = new Date(Number(m.internalDate)).toLocaleString();
        btn.innerHTML = `\n            <div class=\"text-sm text-gray-600\">${date}</div>\n            <div class=\"text-sm text-gray-600\">${m.from}</div>\n            <div class=\"font-medium break-words\">${m.subject}</div>\n            <div class=\"text-xs text-gray-500\">${m.preview}</div>`;
        btn.onclick = () => {
          player.src = `${API}/messages/${m.id}/tts/stream`;
          player.play();
        };
        const li = document.createElement('li');
        li.appendChild(btn);
        listEl.appendChild(li);
      });
    })
    .catch(err => console.error(err));
}

filterForm.addEventListener('submit', (e) => {
  e.preventDefault();
  fetchList();
});

document.addEventListener('DOMContentLoaded', fetchList); 