// 设计稿预览脚本：页面切换条、首页与详情切换、弹窗、标签页、单选组、二维码占位
(function () {
  var pages = [
    ['首页', 'index.html#home'], ['资源详情', 'index.html#detail'], ['分类列表', 'category.html'],
    ['用户中心', 'user.html'], ['会员开通', 'vip.html'], ['支付弹窗', 'vip.html#pay'], ['登录注册', 'login.html']
  ];
  var bar = document.createElement('nav');
  bar.className = 'pv';
  bar.setAttribute('aria-label', '设计稿页面切换');
  bar.innerHTML = pages.map(function (p) { return '<a href="' + p[1] + '">' + p[0] + '</a>'; }).join('');
  document.body.appendChild(bar);

  function file() { return location.pathname.split('/').pop() || 'index.html'; }

  function closeModals() {
    document.querySelectorAll('.modal').forEach(function (m) { m.hidden = true; });
    document.documentElement.style.overflow = '';
  }
  function openModal(id) {
    var m = document.getElementById(id);
    if (!m) return;
    closeModals();
    m.hidden = false;
    document.documentElement.style.overflow = 'hidden';
    var d = m.querySelector('.dialog');
    if (d) { d.setAttribute('tabindex', '-1'); d.focus({ preventScroll: true }); }
  }

  function sync(scroll) {
    var h = location.hash, f = file();
    var sections = document.querySelectorAll('[data-page]');
    if (sections.length) {
      var p = h === '#detail' ? 'detail' : 'home';
      sections.forEach(function (s) { s.hidden = s.dataset.page !== p; });
    }
    closeModals();
    if (h && h.length > 1 && document.getElementById(h.slice(1)) && document.getElementById(h.slice(1)).classList.contains('modal')) openModal(h.slice(1));
    bar.querySelectorAll('a').forEach(function (a) {
      var parts = a.getAttribute('href').split('#'), hf = parts[0], hh = parts[1] ? '#' + parts[1] : '';
      var on = hf === f && (hh ? (hh === h || (hh === '#home' && h !== '#detail')) : h !== '#pay');
      a.classList.toggle('on', on);
    });
    if (scroll) window.scrollTo(0, 0);
  }
  addEventListener('hashchange', function () { sync(location.hash !== '#pay'); });
  sync(false);

  document.addEventListener('click', function (e) {
    var t = e.target;
    var open = t.closest('[data-open]');
    if (open) { e.preventDefault(); openModal(open.dataset.open); return; }
    if (t.closest('[data-close]') || (t.classList && t.classList.contains('modal'))) {
      e.preventDefault();
      closeModals();
      if (location.hash === '#pay') history.replaceState(null, '', location.pathname);
      sync(false);
      return;
    }
    var tab = t.closest('[data-tab]');
    if (tab) {
      e.preventDefault();
      var group = tab.closest('[data-tabs]') || document.querySelector('[data-tabs]');
      group.querySelectorAll('[data-tab]').forEach(function (x) { x.classList.toggle('on', x.dataset.tab === tab.dataset.tab); });
      var root = document.querySelector(group.dataset.tabs);
      if (root) root.querySelectorAll('[data-pane]').forEach(function (p) { p.hidden = p.dataset.pane !== tab.dataset.tab; });
      return;
    }
    var pick = t.closest('[data-pick] > button:not(:disabled)');
    if (pick) {
      pick.parentElement.querySelectorAll(':scope > button').forEach(function (b) { b.classList.toggle('on', b === pick); });
    }
  });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && document.querySelector('.modal:not([hidden])')) {
      closeModals();
      if (location.hash === '#pay') history.replaceState(null, '', location.pathname);
      sync(false);
    }
  });

  // 付款二维码占位图（非真实二维码）
  document.querySelectorAll('.qr').forEach(function (el, idx) {
    var n = 25, seed = 7 + idx, s = '<svg viewBox="0 0 ' + n + ' ' + n + '" shape-rendering="crispEdges" aria-hidden="true">';
    function rnd() { seed = (seed * 9301 + 49297) % 233280; return seed / 233280; }
    function finder(x, y) {
      return '<rect x="' + x + '" y="' + y + '" width="7" height="7"/><rect x="' + (x + 1) + '" y="' + (y + 1) + '" width="5" height="5" fill="#fff"/><rect x="' + (x + 2) + '" y="' + (y + 2) + '" width="3" height="3"/>';
    }
    for (var y = 0; y < n; y++) for (var x = 0; x < n; x++) {
      if ((x < 8 && y < 8) || (x > n - 9 && y < 8) || (x < 8 && y > n - 9)) continue;
      if (rnd() > 0.52) s += '<rect x="' + x + '" y="' + y + '" width="1" height="1"/>';
    }
    el.innerHTML = s + finder(0, 0) + finder(n - 7, 0) + finder(0, n - 7) + '</svg>';
  });
})();
