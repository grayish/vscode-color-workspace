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

function styleOf(bg) {
  return {
    bg: getBgHex(bg),
    bgHover: getHover(bg),
    inactiveBg: getInactiveBg(bg),
    fg: getFg(bg),
    inactiveFg: getInactiveFg(bg),
    badgeBg: getReadableAccent(bg, 2),
  };
}

function adjustedOf(bg, adjustment) {
  if (!adjustment || adjustment === 'none') return bg;
  const c = tinycolor(bg);
  if (adjustment === 'lighten') return formatHex(c.lighten());
  if (adjustment === 'darken')  return formatHex(c.darken());
  return bg;
}

function prepareColors(bg, opts) {
  const out = {};
  const adj = opts.elementAdjustments || {};
  const titleStyle    = styleOf(adjustedOf(bg, adj.titleBar));
  const activityStyle = styleOf(adjustedOf(bg, adj.activityBar));
  const statusStyle   = styleOf(adjustedOf(bg, adj.statusBar));
  const debugBg = complement(statusStyle.bg);

  if (opts.titleBar) {
    out['titleBar.activeBackground'] = titleStyle.bg;
    if (opts.statusAndTitleBorders) out['titleBar.border'] = titleStyle.bg;
    out['titleBar.inactiveBackground'] = titleStyle.inactiveBg;
    if (!opts.keepForegroundColor) {
      out['titleBar.activeForeground'] = titleStyle.fg;
      out['titleBar.inactiveForeground'] = titleStyle.inactiveFg;
      out['commandCenter.border'] = titleStyle.inactiveFg;
    }
  }
  if (opts.activityBar) {
    out['activityBar.background'] = activityStyle.bg;
    out['activityBar.activeBackground'] = activityStyle.bg;
    if (!opts.keepForegroundColor) {
      out['activityBar.foreground'] = activityStyle.fg;
      out['activityBar.inactiveForeground'] = activityStyle.inactiveFg;
    }
    if (!opts.keepBadgeColor) {
      out['activityBarBadge.background'] = activityStyle.badgeBg;
      out['activityBarBadge.foreground'] = getFg(activityStyle.badgeBg);
    }
  }
  if (opts.statusBar) {
    out['statusBar.background'] = statusStyle.bg;
    out['statusBarItem.hoverBackground'] = statusStyle.bgHover;
    out['statusBarItem.remoteBackground'] = statusStyle.bg;
    if (opts.statusAndTitleBorders) out['statusBar.border'] = statusStyle.bg;
    if (!opts.keepForegroundColor) {
      out['statusBar.foreground'] = statusStyle.fg;
      out['statusBarItem.remoteForeground'] = statusStyle.fg;
    }
    if (opts.debuggingStatusBar) {
      out['statusBar.debuggingBackground'] = debugBg;
      if (opts.statusAndTitleBorders) out['statusBar.debuggingBorder'] = debugBg;
      if (!opts.keepForegroundColor) out['statusBar.debuggingForeground'] = getFg(debugBg);
    }
  }
  // Accent borders use activityBar's adjusted color (matches Peacock).
  if (opts.editorGroupBorder) out['editorGroup.border'] = activityStyle.bg;
  if (opts.panelBorder) out['panel.border'] = activityStyle.bg;
  if (opts.sideBarBorder) out['sideBar.border'] = activityStyle.bg;
  if (opts.sashHover) out['sash.hoverBorder'] = activityStyle.bg;
  if (opts.tabActiveBorder) out['tab.activeBorder'] = activityStyle.bg;
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
  {
    base: '#5a3b8c',
    label: 'purple_activity_lighten_title_darken',
    opts: defaultOpts,
    adj: { activityBar: 'lighten', titleBar: 'darken' },
  },
];

const output = fixtures.map(f => ({
  base: f.base,
  label: f.label,
  opts: f.opts,
  elementAdjustments: f.adj || {},
  palette: prepareColors(f.base, { ...f.opts, elementAdjustments: f.adj }),
}));

process.stdout.write(JSON.stringify(output, null, 2) + '\n');
