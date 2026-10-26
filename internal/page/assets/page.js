// Runs inside the preview page. Links never navigate the viewer itself:
// in-page anchors scroll, everything else is handed to the editor through
// the mdrOpen binding (resolved against <base>, so relative links arrive as
// absolute file:// URLs).
document.addEventListener('click', (ev) => {
  const a = ev.target.closest('a[href]');
  if (!a) return;
  ev.preventDefault();
  const raw = a.getAttribute('href');
  if (raw.startsWith('#')) {
    const target = document.getElementById(decodeURIComponent(raw.slice(1)));
    if (target) target.scrollIntoView();
    return;
  }
  if (window.mdrOpen) window.mdrOpen(a.href);
});
