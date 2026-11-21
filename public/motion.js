/**
 * Cicada Motion System
 * Palantir-inspired slow, cinematic, tightly eased, restrained motion.
 * No layout thrashing, passive scroll listeners, single rAF loop.
 */

import { initCinematic } from './cinematic.js';

// Global motion state
let isMobile = false;
let isPaused = false;
let cinematicEngine = null;
let cachedMetrics = {
  heroHeight: 0,
  manifestoTop: 0,
  manifestoHeight: 0,
  duplexTop: 0,
  duplexHeight: 0,
  windowHeight: window.innerHeight,
  windowWidth: window.innerWidth
};

let scrollY = window.scrollY;
let lastScrollY = window.scrollY;
let rafId = null;
let ticking = false;

// DOM references
let headerEl = null;
let heroEl = null;
let heroMediaEl = null;
let heroTitleEl = null;
let heroBottomEl = null;
let manifestoEl = null;
let manifestoWords = [];
let duplexEl = null;
let storyTrackEl = null;
let storyProgressBar = null;
let sloganTimer = null;
let isHeroVisible = true;

// Intersection observer for reveals
let revealObserver = null;

/**
 * Cache metrics on resize/orientationchange to prevent per-frame layout reads
 */
function updateMetrics() {
  cachedMetrics.windowHeight = window.innerHeight;
  cachedMetrics.windowWidth = window.innerWidth;
  isMobile = window.innerWidth <= 640;

  if (heroEl) {
    cachedMetrics.heroHeight = heroEl.offsetHeight;
  }

  if (manifestoEl) {
    const rect = manifestoEl.getBoundingClientRect();
    cachedMetrics.manifestoTop = rect.top + window.scrollY;
    cachedMetrics.manifestoHeight = rect.height;
  }

  if (duplexEl) {
    const rect = duplexEl.getBoundingClientRect();
    cachedMetrics.duplexTop = rect.top + window.scrollY;
    cachedMetrics.duplexHeight = rect.height;
  }
}

/**
 * Check if motion should be paused (OS preference or body.paused)
 */
function checkPaused() {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  isPaused = reducedMotion || document.body.classList.contains('paused');
  return isPaused;
}

function stopSloganLoop() {
  if (sloganTimer) {
    clearTimeout(sloganTimer);
    sloganTimer = null;
  }
}

function startSloganLoop() {
  stopSloganLoop();
  if (checkPaused() || !heroTitleEl || !isHeroVisible) return;

  const inners = heroTitleEl.querySelectorAll('.hero-line-inner');
  if (inners.length === 0) return;

  // Dwell for 3.5 seconds while resting in view
  sloganTimer = setTimeout(() => {
    if (checkPaused() || !isHeroVisible) return;

    // 1. Exit: Slide up out of view through top mask with 90ms stagger
    inners.forEach((line, idx) => {
      line.classList.remove('revealed', 'line-in', 'line-reset');
      setTimeout(() => {
        if (checkPaused() || !isHeroVisible) return;
        line.classList.add('line-out');
      }, idx * 90);
    });

    // 2. Reset position below after exit transition completes
    sloganTimer = setTimeout(() => {
      if (checkPaused() || !isHeroVisible) return;

      inners.forEach(line => {
        line.classList.remove('line-out');
        line.classList.add('line-reset');
      });

      // 3. Pause briefly at bottom (400ms) then slide back up into view
      sloganTimer = setTimeout(() => {
        if (checkPaused() || !isHeroVisible) return;

        inners.forEach((line, idx) => {
          line.classList.remove('line-reset');
          setTimeout(() => {
            if (checkPaused() || !isHeroVisible) return;
            line.classList.add('line-in');
          }, idx * 90);
        });

        // 4. Continue permanent loop
        startSloganLoop();
      }, 400);
    }, 1050);
  }, 3500);
}

/**
 * 1. Page-load intro
 */
function setupIntro() {
  if (!headerEl || !heroTitleEl) return;

  // Header slide down
  if (!checkPaused()) {
    headerEl.classList.add('intro-active');
    setTimeout(() => {
      headerEl.classList.remove('intro-active');
    }, 950);
  }

  // Hero H1 line-by-line reveal
  const originalHtml = heroTitleEl.innerHTML;
  const parts = originalHtml.split(/<br\s*\/?>/i);
  if (parts.length > 1) {
    heroTitleEl.innerHTML = parts.map((text, idx) => `
      <span class="hero-line-mask">
        <span class="hero-line-inner" data-line-index="${idx}">${text.trim()}</span>
      </span>
    `).join('');

    // Ensure hero title container is visible for masked reveal
    heroTitleEl.classList.add('is-visible');

    if (checkPaused()) {
      heroTitleEl.querySelectorAll('.hero-line-inner').forEach(line => {
        line.classList.add('revealed', 'line-in');
      });
      if (heroBottomEl) heroBottomEl.classList.add('revealed');
    } else {
      const inners = heroTitleEl.querySelectorAll('.hero-line-inner');
      requestAnimationFrame(() => {
        inners.forEach((line, idx) => {
          setTimeout(() => {
            line.classList.add('revealed', 'line-in');
          }, 120 + idx * 90);
        });

        // Hero bottom fades in last
        if (heroBottomEl) {
          setTimeout(() => {
            heroBottomEl.classList.add('revealed');
          }, 120 + inners.length * 90 + 300);
        }

        // Start permanent slogan appearance loop
        startSloganLoop();
      });
    }
  }
}

/**
 * 4. Manifesto scroll-scrub setup
 */
function setupManifesto() {
  if (!manifestoEl) return;

  const rawHtml = manifestoEl.innerHTML;
  const lines = rawHtml.split(/<br\s*\/?>/i);

  manifestoEl.innerHTML = lines.map(line => {
    const tempDiv = document.createElement('div');
    tempDiv.innerHTML = line.trim();
    const words = (tempDiv.textContent || '').trim().split(/\s+/);
    return words.map(w => `<span class="manifesto-word">${w}</span>`).join(' ');
  }).join('<br>');

  manifestoWords = Array.from(manifestoEl.querySelectorAll('.manifesto-word'));

  if (checkPaused()) {
    manifestoWords.forEach(w => { w.style.opacity = '1'; });
  }
}

/**
 * 5. General reveal system (IntersectionObserver with 0.15 threshold)
 */
function setupReveals() {
  if (!('IntersectionObserver' in window)) {
    document.querySelectorAll('.reveal, [data-reveal], [data-reveal-stagger] > *, .product-row, .story, .solution-card').forEach(el => {
      el.classList.add('is-visible');
    });
    return;
  }

  revealObserver = new IntersectionObserver((entries, observer) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('is-visible');
        observer.unobserve(entry.target);
      }
    });
  }, {
    threshold: 0.15,
    rootMargin: '0px 0px -20px 0px'
  });

  // Assign stagger indices to products
  document.querySelectorAll('#product-list .product-row').forEach((row, i) => {
    row.style.setProperty('--product-index', i);
    revealObserver.observe(row);
  });

  // Assign stagger indices to carousel stories
  document.querySelectorAll('#story-track .story').forEach((story, i) => {
    story.style.setProperty('--story-index', i);
    revealObserver.observe(story);
  });

  // Observe solution marquee container
  const solutionMarquee = document.getElementById('solution-marquee');
  if (solutionMarquee) {
    revealObserver.observe(solutionMarquee);
  }
  document.querySelectorAll('.solution-card').forEach(card => {
    card.classList.add('is-visible');
  });

  // Section headings & containers with staggered children
  const staggeredContainers = [
    '.products-heading',
    '.section-heading',
    '.duplex-content',
    'footer .footer-column'
  ];

  staggeredContainers.forEach(sel => {
    document.querySelectorAll(sel).forEach(container => {
      container.setAttribute('data-reveal-stagger', '');
      Array.from(container.children).forEach((child, i) => {
        child.style.setProperty('--reveal-index', i);
      });
      revealObserver.observe(container);
    });
  });

  // General reveal targets
  const targets = document.querySelectorAll(`
    .duplex-image,
    .duplex,
    .manifesto-box,
    .cta-section,
    .feature-toolbar,
    .story-controls,
    .footer-identity
  `);

  targets.forEach((el, index) => {
    el.setAttribute('data-reveal', '');
    el.style.setProperty('--reveal-index', index % 4);
    revealObserver.observe(el);
  });
}

/**
 * 7. Carousel desktop drag-to-scroll with inertia and progress bar
 */
function setupCarousel() {
  if (!storyTrackEl) return;

  // Add progress bar if not present
  if (!document.querySelector('.story-progress')) {
    const progressContainer = document.createElement('div');
    progressContainer.className = 'story-progress';
    progressContainer.setAttribute('aria-hidden', 'true');
    progressContainer.innerHTML = '<div class="story-progress-bar"></div>';
    storyTrackEl.parentNode.insertBefore(progressContainer, storyTrackEl.nextSibling);
    storyProgressBar = progressContainer.querySelector('.story-progress-bar');
  } else {
    storyProgressBar = document.querySelector('.story-progress-bar');
  }

  function updateProgressBar() {
    if (!storyProgressBar || !storyTrackEl) return;
    const maxScroll = storyTrackEl.scrollWidth - storyTrackEl.clientWidth;
    const ratio = maxScroll > 0 ? storyTrackEl.scrollLeft / maxScroll : 0;
    storyProgressBar.style.transform = `scaleX(${Math.max(0, Math.min(1, ratio))})`;
  }

  storyTrackEl.addEventListener('scroll', updateProgressBar, { passive: true });
  updateProgressBar();

  // Desktop Pointer Drag with inertia
  let isDragging = false;
  let startX = 0;
  let scrollStart = 0;
  let velocity = 0;
  let lastX = 0;
  let lastTime = 0;
  let dragDistance = 0;
  let momentumRaf = null;

  storyTrackEl.classList.add('is-draggable');

  storyTrackEl.addEventListener('pointerdown', e => {
    // Only drag with primary mouse button on desktop
    if (e.pointerType !== 'mouse' || e.button !== 0) return;

    cancelAnimationFrame(momentumRaf);
    isDragging = true;
    startX = e.pageX;
    lastX = e.pageX;
    scrollStart = storyTrackEl.scrollLeft;
    lastTime = performance.now();
    dragDistance = 0;
    velocity = 0;

    storyTrackEl.classList.add('is-dragging');
    storyTrackEl.setPointerCapture(e.pointerId);
  });

  storyTrackEl.addEventListener('pointermove', e => {
    if (!isDragging) return;

    const dx = e.pageX - startX;
    dragDistance = Math.abs(dx);
    storyTrackEl.scrollLeft = scrollStart - dx;

    const now = performance.now();
    const dt = now - lastTime;
    if (dt > 10) {
      velocity = (e.pageX - lastX) / dt;
      lastX = e.pageX;
      lastTime = now;
    }
  });

  function stopDrag(e) {
    if (!isDragging) return;
    isDragging = false;
    storyTrackEl.classList.remove('is-dragging');

    try {
      storyTrackEl.releasePointerCapture(e.pointerId);
    } catch (_) {}

    // If dragged significantly, prevent default click on cards
    if (dragDistance > 6) {
      const clickCapturer = ev => {
        ev.stopPropagation();
        ev.preventDefault();
        window.removeEventListener('click', clickCapturer, true);
      };
      window.addEventListener('click', clickCapturer, true);
    }

    // Apply inertia on desktop
    if (Math.abs(velocity) > 0.15) {
      function runMomentum() {
        velocity *= 0.92;
        storyTrackEl.scrollLeft -= velocity * 16;
        if (Math.abs(velocity) > 0.02) {
          momentumRaf = requestAnimationFrame(runMomentum);
        } else {
          storyTrackEl.style.scrollSnapType = '';
        }
      }
      momentumRaf = requestAnimationFrame(runMomentum);
    } else {
      storyTrackEl.style.scrollSnapType = '';
    }
  }

  storyTrackEl.addEventListener('pointerup', stopDrag);
  storyTrackEl.addEventListener('pointercancel', stopDrag);
}

/**
 * 10. Dialogs staggered reveal
 */
export function setupDialogReveals(dialog) {
  if (!dialog) return;

  if (dialog.id === 'navigation-dialog') {
    const navLinks = dialog.querySelectorAll('.main-navigation > *');
    navLinks.forEach((el, i) => {
      el.style.setProperty('--nav-link-index', i);
    });
  } else if (dialog.id === 'content-dialog') {
    const items = dialog.querySelectorAll(`
      #dialog-title,
      .dialog-lead,
      .detail-hero,
      .capability-list li,
      .catalog-grid .catalog-item,
      .dialog-note,
      .dialog-actions > *,
      .arch-layer,
      .service,
      .deployment-tabs,
      .deployment-panel > *
    `);

    items.forEach((item, i) => {
      item.classList.add('dialog-reveal-item');
      item.style.setProperty('--dialog-reveal-index', Math.min(i, 12));
    });

    requestAnimationFrame(() => {
      items.forEach(item => {
        item.classList.add('is-visible');
      });
    });
  }
}

/**
 * Main rAF update loop for scroll-driven animations
 * No layout reads take place here!
 */
function onScrollFrame() {
  ticking = false;
  checkPaused();

  if (isPaused) {
    if (headerEl) {
      headerEl.classList.remove('header-hidden');
      headerEl.classList.toggle('scrolled', scrollY > 80);
    }
    return;
  }

  // 3. Header behavior: statically visible floating pill bar all the way to the end of the page
  if (headerEl) {
    headerEl.classList.toggle('scrolled', scrollY > 80);
    headerEl.classList.remove('header-hidden');
    lastScrollY = scrollY;
  }

  // 2. Hero scroll parallax & visibility
  const heroInView = scrollY <= (cachedMetrics.heroHeight || 600) + 50;
  if (heroInView !== isHeroVisible) {
    isHeroVisible = heroInView;
    if (isHeroVisible) {
      startSloganLoop();
    } else {
      stopSloganLoop();
    }
  }

  if (!isMobile && heroEl && heroInView) {
    const heroHeight = cachedMetrics.heroHeight || 600;

    // .hero-media translateY at ~0.25x scroll speed
    const mediaY = (scrollY * 0.25).toFixed(1);
    heroEl.style.setProperty('--hero-parallax-y', `${mediaY}px`);

    // Hero h1 translateY slightly faster (~0.38x) and fades to 0 by ~60% hero height
    const h1Y = (scrollY * 0.38).toFixed(1);
    const h1Opacity = Math.max(0, 1 - scrollY / (heroHeight * 0.6)).toFixed(3);
    heroEl.style.setProperty('--hero-h1-y', `${h1Y}px`);
    heroEl.style.setProperty('--hero-h1-opacity', h1Opacity);

    // .hero-shade darkens gradually
    const shadeDarken = Math.min(0.65, (scrollY / heroHeight) * 0.65).toFixed(3);
    heroEl.style.setProperty('--hero-shade-darken', shadeDarken);
  }

  // 4. Manifesto scroll-scrub text
  if (manifestoWords.length > 0) {
    const { manifestoTop, manifestoHeight, windowHeight } = cachedMetrics;
    const start = manifestoTop - windowHeight * 0.75;
    const end = manifestoTop + manifestoHeight - windowHeight * 0.35;

    if (scrollY + windowHeight >= start && scrollY <= end + windowHeight) {
      const progress = Math.max(0, Math.min(1, (scrollY - start) / (end - start)));
      const count = manifestoWords.length;

      for (let i = 0; i < count; i++) {
        const threshold = i / count;
        const wordProgress = Math.max(0, Math.min(1, (progress - threshold) * count));
        const wordOpacity = 0.15 + 0.85 * wordProgress;
        manifestoWords[i].style.opacity = wordOpacity.toFixed(2);
      }
    } else if (scrollY + windowHeight < start) {
      for (let i = 0; i < manifestoWords.length; i++) {
        manifestoWords[i].style.opacity = '0.15';
      }
    } else {
      for (let i = 0; i < manifestoWords.length; i++) {
        manifestoWords[i].style.opacity = '1';
      }
    }
  }

  // 8. Duplex subtle scroll parallax
  if (!isMobile && duplexEl) {
    const { duplexTop, duplexHeight, windowHeight } = cachedMetrics;
    if (scrollY + windowHeight >= duplexTop && scrollY <= duplexTop + duplexHeight + windowHeight) {
      const progress = (scrollY + windowHeight - duplexTop) / (windowHeight + duplexHeight);
      const parallax = ((progress - 0.5) * 32).toFixed(1);
      duplexEl.style.setProperty('--duplex-parallax', `${parallax}px`);
    }
  }
}

/**
 * Passive scroll event handler
 */
function onScroll() {
  scrollY = window.scrollY;
  if (!ticking) {
    rafId = requestAnimationFrame(onScrollFrame);
    ticking = true;
  }
}

/**
 * Handle state changes when reduced motion / pause toggles
 */
function handleMotionStateChange() {
  const paused = checkPaused();
  if (cinematicEngine) {
    cinematicEngine.setPausedState(paused);
  }
  if (paused) {
    stopSloganLoop();
    if (heroTitleEl) {
      heroTitleEl.querySelectorAll('.hero-line-inner').forEach(line => {
        line.classList.remove('line-out', 'line-reset');
        line.classList.add('line-in', 'revealed');
      });
    }
    if (headerEl) {
      headerEl.classList.remove('header-hidden');
    }
    if (heroEl) {
      heroEl.style.setProperty('--hero-parallax-y', '0px');
      heroEl.style.setProperty('--hero-h1-y', '0px');
      heroEl.style.setProperty('--hero-h1-opacity', '1');
      heroEl.style.setProperty('--hero-shade-darken', '0');
    }
    manifestoWords.forEach(w => {
      w.style.opacity = '1';
    });
    document.querySelectorAll('.reveal, [data-reveal], [data-reveal-stagger] > *, .product-row, .story, .solution-card').forEach(el => {
      el.classList.add('is-visible');
    });
  } else {
    updateMetrics();
    onScroll();
    startSloganLoop();
  }
}

/**
 * Primary initializer
 */
export function initMotion() {
  headerEl = document.getElementById('header');
  heroEl = document.querySelector('.hero');
  heroMediaEl = document.getElementById('hero-media');
  heroTitleEl = document.getElementById('hero-title');
  heroBottomEl = document.querySelector('.hero-bottom');
  manifestoEl = document.querySelector('.manifesto');
  duplexEl = document.querySelector('.duplex-image');
  storyTrackEl = document.getElementById('story-track');

  setupIntro();
  setupManifesto();
  updateMetrics();
  setupReveals();
  setupCarousel();

  // Initialize WebGL cinematic telemetry mesh
  cinematicEngine = initCinematic();

  // Navigation dialog links stagger setup
  const navDialog = document.getElementById('navigation-dialog');
  if (navDialog) {
    setupDialogReveals(navDialog);
  }

  // Passive event listeners
  window.addEventListener('scroll', onScroll, { passive: true });
  window.addEventListener('resize', () => {
    updateMetrics();
    onScroll();
  }, { passive: true });
  window.addEventListener('orientationchange', () => {
    updateMetrics();
    onScroll();
  }, { passive: true });

  // Watch for paused / reduced motion changes
  const reducedMotionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
  reducedMotionQuery.addEventListener('change', handleMotionStateChange);

  const observer = new MutationObserver(mutations => {
    for (const mutation of mutations) {
      if (mutation.type === 'attributes' && mutation.attributeName === 'class') {
        handleMotionStateChange();
      }
    }
  });
  observer.observe(document.body, { attributes: true, attributeFilter: ['class'] });

  // Tab visibility changes to pause loop when tab is backgrounded
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      stopSloganLoop();
    } else if (isHeroVisible && !checkPaused()) {
      startSloganLoop();
    }
  });

  // Initial scroll evaluation
  onScroll();
}
