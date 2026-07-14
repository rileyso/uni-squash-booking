(() => {
  const form = document.querySelector('[data-attendance-form]');
  if (!form) return;
  const start = form.querySelector('[data-start]');
  const end = form.querySelector('[data-end]');
  const startRange = form.querySelector('[data-start-range]');
  const endRange = form.querySelector('[data-end-range]');
  const summary = form.querySelector('[data-summary]');
  const label = minutes => {
    if (minutes === 1440) return '12:00 am';
    const hour = Math.floor(minutes / 60);
    const minute = minutes % 60;
    return new Date(2000, 0, 1, hour, minute).toLocaleTimeString([], {hour:'numeric', minute:'2-digit'});
  };
  const sync = source => {
    if (source === startRange) start.value = startRange.value;
    if (source === endRange) end.value = endRange.value;
    if (source === start) startRange.value = start.value;
    if (source === end) endRange.value = end.value;
    let a = Number(start.value), b = Number(end.value);
    if (b <= a) { b = Math.min(1440, a + 30); end.value = b; endRange.value = b; }
    const duration = b - a;
    summary.textContent = `${label(a)}–${label(b)} (${duration / 60 % 1 ? (duration / 60).toFixed(1) : duration / 60} hours)`;
  };
  [start,end,startRange,endRange].forEach(control => control.addEventListener('input', () => sync(control)));
  sync();
})();
