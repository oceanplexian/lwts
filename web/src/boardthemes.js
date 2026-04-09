const APPEARANCE_CACHE_KEY = 'lwts-appearance';

const BOARD_THEMES = Object.freeze({
  default: {
    id: 'default',
    label: 'Default',
    description: 'Clean graphite workspace with a subtle blue glow',
  },
  'summer-duotone': {
    id: 'summer-duotone',
    label: 'Summer Duotone',
    description: 'Warm tropical print in sunset reds and soft cream',
  },
  'round-geometric': {
    id: 'round-geometric',
    label: 'Round Geometric',
    description: 'Minimal charcoal pattern with warm stone circles and lines',
  },
  squares: {
    id: 'squares',
    label: 'Squares',
    description: 'Electric blue pixel squares across a deep midnight grid',
  },
  'space-doodles': {
    id: 'space-doodles',
    label: 'Space Doodles',
    description: 'Hand-drawn planets, rockets, and stars on an ink black sky',
  },
  'deep-cosmos': {
    id: 'deep-cosmos',
    label: 'Deep Cosmos',
    description: 'Meteor trails and orbit linework across a midnight blue field',
  },
  space: {
    id: 'space',
    label: 'Space / Stars',
    description: 'Deep nebula tones with crisp stars and cosmic bloom',
  },
  aurora: {
    id: 'aurora',
    label: 'Aurora',
    description: 'Emerald and violet light bands across a polar night sky',
  },
  blueprint: {
    id: 'blueprint',
    label: 'Blueprint',
    description: 'Drafting grid with luminous cyan technical lines',
  },
});

function parseBoardSettings(settings) {
  if (!settings) return {};
  if (typeof settings === 'object') return settings;
  try {
    const parsed = JSON.parse(settings);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function resolveBoardThemeId(themeId) {
  return BOARD_THEMES[themeId] ? themeId : 'default';
}

function getAppearanceThemeId(settings) {
  if (!settings || typeof settings !== 'object') return 'default';
  return resolveBoardThemeId(settings.theme);
}

function readCachedAppearanceSettings() {
  try {
    const raw = localStorage.getItem(APPEARANCE_CACHE_KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function getCachedBoardThemeId() {
  return getAppearanceThemeId(readCachedAppearanceSettings());
}

function applyBoardTheme(themeId) {
  const resolvedThemeId = resolveBoardThemeId(themeId);
  if (document.body) {
    document.body.dataset.boardTheme = resolvedThemeId;
  }
  return resolvedThemeId;
}

function syncCurrentBoardTheme() {
  return applyBoardTheme(getCachedBoardThemeId());
}

window.BOARD_THEMES = BOARD_THEMES;
window.parseBoardSettings = parseBoardSettings;
window.resolveBoardThemeId = resolveBoardThemeId;
window.getAppearanceThemeId = getAppearanceThemeId;
window.getCachedBoardThemeId = getCachedBoardThemeId;
window.applyBoardTheme = applyBoardTheme;
window.syncCurrentBoardTheme = syncCurrentBoardTheme;
