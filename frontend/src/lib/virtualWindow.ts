/** Slice a long list to the visible window (+ overscan) for cheap scrolling. */

export function virtualWindow(opts: {
  count: number;
  scrollTop: number;
  viewportH: number;
  itemH: number;
  overscan?: number;
}): { start: number; end: number; padTop: number; totalH: number } {
  const itemH = Math.max(1, Math.floor(opts.itemH));
  const overscan = opts.overscan ?? 8;
  const count = Math.max(0, Math.floor(opts.count));
  const scrollTop = Math.max(0, opts.scrollTop);
  const viewportH = Math.max(0, opts.viewportH);
  const start = Math.min(count, Math.max(0, Math.floor(scrollTop / itemH) - overscan));
  const visible = Math.ceil(viewportH / itemH) + overscan;
  const end = Math.min(count, start + visible + overscan);
  return {
    start,
    end,
    padTop: start * itemH,
    totalH: count * itemH,
  };
}
