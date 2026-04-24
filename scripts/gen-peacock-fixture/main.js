// Generate golden-test fixtures by re-implementing Peacock's prepareColors
// logic using tinycolor2 (which Peacock itself uses internally).
//
// Usage (from repo root):
//   node scripts/gen-peacock-fixture/main.js > internal/color/testdata/fixture.json

const tinycolor = require('tinycolor2');

const inactiveAlpha = 0x99 / 0xff;
const defaultSaturation = 0.5;
const darkFg = '#15202b';
const lightFg = '#e7e7e7';

function formatHex(c) {
  return c.getAlpha() < 1 ? c.toHex8String() : c.toHexString();
}

function getBgHex(c) { return formatHex(tinycolor(c)); }

function getInactiveBg(c) {
  const x = tinycolor(c);
  x.setAlpha(inactiveAlpha);
  return formatHex(x);
}

function getHover(c) {
  const x = tinycolor(c);
  return formatHex(x.isLight() ? x.darken() : x.lighten());
}

function getFg(bg) {
  const x = tinycolor(bg);
  return formatHex(tinycolor(x.isLight() ? darkFg : lightFg));
}

function getInactiveFg(bg) {
  const f = tinycolor(getFg(bg));
  f.setAlpha(inactiveAlpha);
  return formatHex(f);
}

function getReadableAccent(bg, ratio) {
  const background = tinycolor(bg);
  const fg = background.triad()[1];
  let { h, s, l } = fg.toHsl();
  if (s === 0) h = 60 * Math.round(l * 6);
  if (s < 0.15) s = defaultSaturation;
  const count = 16;
  const shades = [...Array(count).keys()].map(i => {
    const c = tinycolor({ h, s, l: i / count });
    return { contrast: tinycolor.readability(c, background), hex: formatHex(c) };
  });
  shades.sort((a, b) => a.contrast - b.contrast);
  const found = shades.find(s => s.contrast >= ratio);
  return found ? found.hex : '#ffffff';
}

function complement(c) { return formatHex(tinycolor(c).complement()); }

function elementStyle(bg) {
  return {
    bg: getBgHex(bg),
    bgHover: getHover(bg),
    inactiveBg: getInactiveBg(bg),
    fg: getFg(bg),
    inactiveFg: getInactiveFg(bg),
    badgeBg: getReadableAccent(bg, 2),
  };
}

function prepareColors(bg, opts) {
  const out = {};
  const style = elementStyle(bg);
  const debugBg = complement(bg);

  if (opts.titleBar) {
    out['titleBar.activeBackground'] = style.bg;
    if (opts.statusAndTitleBorders) out['titleBar.border'] = style.bg;
    out['titleBar.inactiveBackground'] = style.inactiveBg;
    if (!opts.keepForegroundColor) {
      out['titleBar.activeForeground'] = style.fg;
      out['titleBar.inactiveForeground'] = style.inactiveFg;
      out['commandCenter.border'] = style.inactiveFg;
    }
  }
  if (opts.activityBar) {
    out['activityBar.background'] = style.bg;
    out['activityBar.activeBackground'] = style.bg;
    if (!opts.keepForegroundColor) {
      out['activityBar.foreground'] = style.fg;
      out['activityBar.inactiveForeground'] = style.inactiveFg;
    }
    if (!opts.keepBadgeColor) {
      out['activityBarBadge.background'] = style.badgeBg;
      out['activityBarBadge.foreground'] = getFg(style.badgeBg);
    }
  }
  if (opts.statusBar) {
    out['statusBar.background'] = style.bg;
    out['statusBarItem.hoverBackground'] = style.bgHover;
    out['statusBarItem.remoteBackground'] = style.bg;
    if (opts.statusAndTitleBorders) out['statusBar.border'] = style.bg;
    if (!opts.keepForegroundColor) {
      out['statusBar.foreground'] = style.fg;
      out['statusBarItem.remoteForeground'] = style.fg;
    }
    if (opts.debuggingStatusBar) {
      out['statusBar.debuggingBackground'] = debugBg;
      if (opts.statusAndTitleBorders) out['statusBar.debuggingBorder'] = debugBg;
      if (!opts.keepForegroundColor) out['statusBar.debuggingForeground'] = getFg(debugBg);
    }
  }
  if (opts.editorGroupBorder) out['editorGroup.border'] = style.bg;
  if (opts.panelBorder) out['panel.border'] = style.bg;
  if (opts.sideBarBorder) out['sideBar.border'] = style.bg;
  if (opts.sashHover) out['sash.hoverBorder'] = style.bg;
  if (opts.tabActiveBorder) out['tab.activeBorder'] = style.bg;
  if (opts.squigglyBeGone) {
    out['editorError.foreground'] = '#00000000';
    out['editorWarning.foreground'] = '#00000000';
    out['editorInfo.foreground'] = '#00000000';
  }
  return out;
}

const defaultOpts = {
  activityBar: true, statusBar: true, titleBar: true,
  editorGroupBorder: false, panelBorder: false, sideBarBorder: false,
  sashHover: false, statusAndTitleBorders: false,
  debuggingStatusBar: false, tabActiveBorder: false,
  keepForegroundColor: false, keepBadgeColor: false, squigglyBeGone: false,
};

const fixtures = [
  { base: '#ff0000', label: 'red', opts: defaultOpts },
  { base: '#42b883', label: 'peacock_green', opts: defaultOpts },
  { base: '#5a3b8c', label: 'purple', opts: defaultOpts },
  { base: '#000000', label: 'black', opts: defaultOpts },
  { base: '#ffffff', label: 'white', opts: defaultOpts },
];

const output = fixtures.map(f => ({
  base: f.base,
  label: f.label,
  opts: f.opts,
  palette: prepareColors(f.base, f.opts),
}));

process.stdout.write(JSON.stringify(output, null, 2) + '\n');
