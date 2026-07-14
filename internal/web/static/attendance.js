(() => {
  const form = document.querySelector('[data-attendance-form]');
  if (!form) return;
  const startRange = form.querySelector('[data-start-range]');
  const endRange = form.querySelector('[data-end-range]');
  const timeline = form.querySelector('[data-timeline]');
  const summary = form.querySelector('[data-summary]');
  const minimum = 600;
  const maximum = 1320;
  const allowedMinimum = Number(timeline.dataset.allowedStart);
  const allowedMaximum = Number(timeline.dataset.allowedEnd);
  const minimumGap = 60;
  const label = minutes => {
    if (minutes === 1440) return '12:00 am';
    const hour = Math.floor(minutes / 60);
    const minute = minutes % 60;
    return new Date(2000, 0, 1, hour, minute).toLocaleTimeString([], {hour:'numeric', minute:'2-digit'});
  };
  const sync = source => {
    let a = Number(startRange.value), b = Number(endRange.value);
    if (source === startRange) a = Math.min(a, b - minimumGap);
    if (source === endRange) b = Math.max(b, a + minimumGap);
    a = Math.max(allowedMinimum, Math.min(a, allowedMaximum - minimumGap));
    b = Math.min(allowedMaximum, Math.max(b, a + minimumGap));
    startRange.value = a;
    endRange.value = b;
    startRange.setAttribute('aria-valuetext', label(a));
    endRange.setAttribute('aria-valuetext', label(b));
    const duration = b - a;
    timeline.style.setProperty('--start-position', `${(a - minimum) / (maximum - minimum) * 100}%`);
    timeline.style.setProperty('--end-position', `${(b - minimum) / (maximum - minimum) * 100}%`);
    summary.textContent = `${label(a)}–${label(b)} (${duration / 60 % 1 ? (duration / 60).toFixed(1) : duration / 60} hours)`;
  };
  [startRange,endRange].forEach(control => control.addEventListener('input', () => sync(control)));
  sync();
})();
