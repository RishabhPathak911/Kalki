/**
 * KALKI Hero Scenes Crossfade Controller
 * Palantir-inspired cinematic crossfade across 3 operational atmospheric scenes:
 * 1. WebGL Procedural Telemetry Canvas (Dark night with neon particle network)
 * 2. Amber Glow Operations (Warm amber)
 * 3. Cool Steel Infrastructure (Deep blue/steel)
 */

// TODO: To drop in real video footage:
// 1. Place scene-02.mp4 and scene-02.webp in public/assets/hero/
// 2. Place scene-03.mp4 and scene-03.webp in public/assets/hero/
// 3. Set hasFootage: true on scene-02 and scene-03 below.
export const HERO_SCENES = [
  {
    id: 'scene-01',
    type: 'canvas',
    name: 'Neural Telemetry Mesh',
    shade: 0.35
  },
  {
    id: 'scene-02',
    type: 'video',
    src: '/assets/hero/scene-02.mp4',
    poster: '/assets/hero/scene-02.webp',
    hasFootage: false, // TODO: set to true once scene-02.mp4 is placed in /assets/hero/
    placeholderClass: 'hero-scene-amber',
    shade: 0.55,
    name: 'Amber Glow Operations'
  },
  {
    id: 'scene-03',
    type: 'video',
    src: '/assets/hero/scene-03.mp4',
    poster: '/assets/hero/scene-03.webp',
    hasFootage: false, // TODO: set to true once scene-03.mp4 is placed in /assets/hero/
    placeholderClass: 'hero-scene-steel',
    shade: 0.50,
    name: 'Cool Steel Network'
  }
];

let activeIndex = 0;
let crossfadeTimer = null;
let isPaused = false;
let isOffscreen = false;
let sceneElements = [];
let heroElement = null;
let heroShadeElement = null;

function isMotionPaused() {
  const reducedMotion = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const bodyPaused = typeof document !== 'undefined' && document.body.classList.contains('paused');
  return reducedMotion || bodyPaused;
}

function updateScenePlayback() {
  const shouldPlay = !isPaused && !isOffscreen && !isMotionPaused();

  sceneElements.forEach((el, index) => {
    const video = el.querySelector('video');
    if (video) {
      if (shouldPlay && (index === activeIndex || index === (activeIndex + 1) % HERO_SCENES.length)) {
        video.play().catch(() => {});
      } else {
        video.pause();
      }
    }
  });

  if (shouldPlay) {
    startCrossfadeTimer();
  } else {
    stopCrossfadeTimer();
  }
}

function stopCrossfadeTimer() {
  if (crossfadeTimer) {
    clearTimeout(crossfadeTimer);
    crossfadeTimer = null;
  }
}

function startCrossfadeTimer() {
  stopCrossfadeTimer();
  if (isPaused || isOffscreen || isMotionPaused()) return;

  crossfadeTimer = setTimeout(() => {
    goToScene((activeIndex + 1) % HERO_SCENES.length);
  }, 5000);
}

export function goToScene(nextIndex) {
  if (!sceneElements.length) return;
  const targetIndex = ((nextIndex % HERO_SCENES.length) + HERO_SCENES.length) % HERO_SCENES.length;
  if (targetIndex === activeIndex && sceneElements[activeIndex]?.classList.contains('is-active')) {
    return;
  }

  const prevIndex = activeIndex;
  activeIndex = targetIndex;

  sceneElements.forEach((el, idx) => {
    if (idx === activeIndex) {
      el.classList.add('is-active');
      const video = el.querySelector('video');
      if (video && !isPaused && !isOffscreen && !isMotionPaused()) {
        video.play().catch(() => {});
      }
    } else {
      el.classList.remove('is-active');
      if (idx !== prevIndex) {
        const video = el.querySelector('video');
        if (video) video.pause();
      }
    }
  });

  // Apply per-scene shade strength for minimum 4.5:1 text contrast
  const targetShade = HERO_SCENES[activeIndex]?.shade || 0.35;
  if (heroElement) {
    heroElement.style.setProperty('--hero-scene-shade', String(targetShade));
  }
  if (heroShadeElement) {
    heroShadeElement.style.setProperty('--hero-scene-shade', String(targetShade));
  }

  // After 850ms transition, pause previous scene video
  setTimeout(() => {
    if (prevIndex !== activeIndex) {
      const prevVideo = sceneElements[prevIndex]?.querySelector('video');
      if (prevVideo) prevVideo.pause();
    }
  }, 850);

  startCrossfadeTimer();
}

export function setHeroScenesPaused(paused) {
  isPaused = paused;
  if (isMotionPaused()) {
    // Show Scene 1 statically with no crossfade
    goToScene(0);
  }
  updateScenePlayback();
}

export function initHeroScenes() {
  heroElement = document.querySelector('.hero');
  heroShadeElement = document.querySelector('.hero-shade');
  const heroMedia = document.getElementById('hero-media');
  if (!heroMedia) return;

  let scenesContainer = heroMedia.querySelector('.hero-scenes');
  if (!scenesContainer) {
    scenesContainer = document.createElement('div');
    scenesContainer.className = 'hero-scenes';
    heroMedia.appendChild(scenesContainer);
  }

  const existingCanvas = document.getElementById('hero-canvas');
  const isMobile = window.innerWidth <= 640;
  const prefersReducedData = window.matchMedia('(prefers-reduced-data: reduce)').matches;
  const allowVideo = !isMobile && !prefersReducedData;

  // Scene 1 is built eagerly
  scenesContainer.innerHTML = '';
  sceneElements = HERO_SCENES.map((scene, index) => {
    const sceneEl = document.createElement('div');
    sceneEl.className = `hero-scene ${index === 0 ? 'is-active' : ''}`;
    sceneEl.dataset.sceneIndex = String(index);

    if (scene.type === 'canvas') {
      if (existingCanvas) {
        sceneEl.appendChild(existingCanvas);
      }
    } else if (scene.type === 'video' && scene.hasFootage && allowVideo) {
      const video = document.createElement('video');
      video.className = 'hero-video';
      video.muted = true;
      video.loop = true;
      video.playsInline = true;
      video.preload = 'metadata';
      video.poster = scene.poster || '';
      video.src = scene.src;
      sceneEl.appendChild(video);
    } else {
      const poster = document.createElement('div');
      poster.className = `hero-scene-poster ${scene.placeholderClass || ''}`;
      sceneEl.appendChild(poster);
    }

    scenesContainer.appendChild(sceneEl);
    return sceneEl;
  });

  // Initial shade
  const initialShade = HERO_SCENES[0]?.shade || 0.35;
  if (heroElement) {
    heroElement.style.setProperty('--hero-scene-shade', String(initialShade));
  }
  if (heroShadeElement) {
    heroShadeElement.style.setProperty('--hero-scene-shade', String(initialShade));
  }

  // Load secondary scenes non-eagerly after window load
  const onReady = () => {
    updateScenePlayback();
  };

  if (document.readyState === 'complete') {
    onReady();
  } else {
    window.addEventListener('load', onReady, { once: true });
  }

  // IntersectionObserver to pause off-screen
  if ('IntersectionObserver' in window && heroElement) {
    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        isOffscreen = !entry.isIntersecting;
        updateScenePlayback();
      });
    }, { threshold: 0.1 });
    observer.observe(heroElement);
  }

  // Document visibility
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      stopCrossfadeTimer();
    } else {
      updateScenePlayback();
    }
  });

  // Check initial reduced motion & paused state
  setHeroScenesPaused(isMotionPaused());
}
