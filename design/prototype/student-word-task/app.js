const words = [
  {
    id: "w-apple",
    text: "apple",
    meaningZh: "苹果",
    phonetic: "/ˈæpl/",
    type: "new",
    status: "unlearned",
    correctStreak: 0,
    lane: "first",
    stage: "初学",
    example: "I eat an apple after lunch."
  },
  {
    id: "w-desk",
    text: "desk",
    meaningZh: "书桌",
    phonetic: "/desk/",
    type: "new",
    status: "unlearned",
    correctStreak: 0,
    lane: "first",
    stage: "初学",
    example: "My book is on the desk."
  },
  {
    id: "w-water",
    text: "water",
    meaningZh: "水",
    phonetic: "/ˈwɔːtər/",
    type: "new",
    status: "unlearned",
    correctStreak: 2,
    lane: "first",
    stage: "初学",
    example: "I drink water after running."
  },
  {
    id: "w-read",
    text: "read",
    meaningZh: "阅读",
    phonetic: "/riːd/",
    type: "mistake",
    status: "reinforce",
    correctStreak: 0,
    lane: "review",
    stage: "1天复习",
    example: "We read a story in the morning."
  },
  {
    id: "w-climb",
    text: "climb",
    meaningZh: "攀爬",
    phonetic: "/klaɪm/",
    type: "mistake",
    status: "reinforce",
    correctStreak: 1,
    lane: "review",
    stage: "2天复习",
    example: "The children climb the small hill."
  },
  {
    id: "w-their",
    text: "their",
    meaningZh: "他们的",
    phonetic: "/ðer/",
    type: "mistake",
    status: "reinforce",
    correctStreak: 0,
    lane: "review",
    stage: "4天复习",
    example: "Their bags are under the desk."
  },
  {
    id: "w-school",
    text: "school",
    meaningZh: "学校",
    phonetic: "/skuːl/",
    type: "new",
    status: "learning",
    correctStreak: 1,
    lane: "review",
    stage: "2天复习",
    example: "We go to school in the morning."
  },
  {
    id: "w-small",
    text: "small",
    meaningZh: "小的",
    phonetic: "/smɔːl/",
    type: "mistake",
    status: "mastered",
    correctStreak: 3,
    lane: "review",
    stage: "30天复习",
    example: "The small cat is under the desk."
  },
  {
    id: "w-morning",
    text: "morning",
    meaningZh: "早晨",
    phonetic: "/ˈmɔːrnɪŋ/",
    type: "new",
    status: "learning",
    correctStreak: 2,
    lane: "review",
    stage: "4天复习",
    example: "I read in the morning."
  }
];

const state = {
  view: "dashboard",
  unlockedViews: new Set(["dashboard"]),
  cardIndex: 0,
  listened: new Set(),
  readDone: new Set(),
  hardWords: new Set(),
  showPhonetic: false,
  submitted: false,
  firstDictationSubmitted: false,
  correctionMode: false,
  correctionTargets: new Set(),
  firstAttemptWrong: new Set(),
  firstAttemptBonus: new Set(),
  checkins: new Set(),
  checkinBonusShown: false,
  answers: {}
};

const typeText = {
  new: "新词",
  mistake: "易错词"
};

const statusText = {
  unlearned: "未学",
  learning: "学习中",
  reinforce: "需强化",
  mastered: "已掌握"
};

const strategy = {
  newPoolMaxCount: 3,
  reviewPoolMaxCount: 9,
  lightDayThreshold: 3
};

const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => Array.from(document.querySelectorAll(selector));

function showToast(message) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.classList.add("show");
  window.setTimeout(() => toast.classList.remove("show"), 2200);
}

function switchView(view) {
  if (!state.unlockedViews.has(view)) return;
  state.view = view;
  $$(".view").forEach((el) => el.classList.remove("active"));
  $(`#${view}View`).classList.add("active");
  renderSteps();

  if (view === "cards") renderCard();
  if (view === "reading") renderStory();
  if (view === "dictation") renderDictation();
  if (view === "done") renderDone();
}

function unlockView(view) {
  state.unlockedViews.add(view);
  renderSteps();
}

function renderSteps() {
  $$(".step").forEach((step) => {
    const unlocked = state.unlockedViews.has(step.dataset.view);
    step.hidden = !unlocked;
    step.disabled = !unlocked;
    step.classList.toggle("active", step.dataset.view === state.view);
  });
}

function renderMissionWords() {
  const firstWords = words.filter((word) => word.lane === "first");
  const reviewWords = words.filter((word) => word.lane === "review");
  $("#todayWordCount").textContent = `${words.length} 个词`;
  $("#firstLaneCount").textContent = `新词池 ${firstWords.length}/${strategy.newPoolMaxCount}`;
  $("#reviewLaneCount").textContent = `非新词池 ${reviewWords.length}/${strategy.reviewPoolMaxCount}`;
  renderLoadTag(reviewWords.length);
  $("#missionWords").innerHTML = `
    <div class="mission-lane">
      <div class="lane-title">
        <strong>新词池</strong>
        <span>从未学状态随机抽取，最多 3 个</span>
      </div>
      <div class="word-grid first-pool">${firstWords.map(renderMissionWord).join("")}</div>
    </div>
    <div class="mission-lane">
      <div class="lane-title">
        <strong>非新词池</strong>
        <span>从学习中、需强化、已掌握中按分数选择，最多 9 个</span>
      </div>
      <div class="word-grid review-pool">${reviewWords.map(renderMissionWord).join("")}</div>
    </div>
  `;
}

function renderLoadTag(reviewCount) {
  let type = "普通日";
  let desc = "今天复习量适中，按顺序完成就好。";
  let tone = "normal";
  if (reviewCount < strategy.lightDayThreshold) {
    type = "轻松日";
    desc = "今天复习词不多，可以轻松完成。";
    tone = "light";
  } else if (reviewCount > strategy.reviewPoolMaxCount) {
    type = "压力日";
    desc = "今天到期词偏多，系统只挑优先级最高的一批。";
    tone = "pressure";
  }
  $("#loadTag").className = `load-tag ${tone}`;
  $("#loadTag").innerHTML = `<strong>${type}</strong><span>${desc}</span>`;
}

function renderMissionWord(word) {
  return `
    <article class="mission-word ${word.type} ${word.lane}">
      <strong>${word.text}</strong>
      <span>${word.meaningZh} · ${word.phonetic}</span>
      <span>${typeText[word.type]}标签 · ${word.stage}</span>
      <span>状态：${statusText[word.status]} · 连对 ${word.correctStreak}/3</span>
    </article>
  `;
}

function renderQueue() {
  $("#queueList").innerHTML = words.map((word, index) => `
    <button class="queue-item ${index === state.cardIndex ? "active" : ""} ${state.readDone.has(word.id) ? "done" : ""}" type="button" data-index="${index}">
      <strong>${index + 1}. ${word.text}</strong>
      <span>${word.meaningZh} · ${typeText[word.type]} · ${statusText[word.status]}</span>
    </button>
  `).join("");

  $$(".queue-item").forEach((item) => {
    item.addEventListener("click", () => {
      state.cardIndex = Number(item.dataset.index);
      renderCard();
    });
  });
}

function renderCard() {
  const word = words[state.cardIndex];
  markWordLearning(word);
  $("#cardProgress").textContent = `${state.cardIndex + 1} / ${words.length}`;
  $("#cardType").textContent = `${typeText[word.type]} · ${word.stage} · ${statusText[word.status]} · 连对 ${word.correctStreak}/3`;
  $("#cardPhonetic").textContent = word.phonetic || "暂无音标";
  $("#cardWord").textContent = word.text;
  $("#cardMeaning").textContent = word.meaningZh;
  $("#cardExample").textContent = word.example || "例句待补充。";
  $("#readDoneBtn").disabled = !state.listened.has(word.id);
  $("#cardHint").textContent = state.listened.has(word.id)
    ? "现在自己读一遍，然后点“我读完了”。"
    : "先听一遍读音，再自己读出来。";
  $("#prevCardBtn").disabled = state.cardIndex === 0;
  $("#nextCardBtn").textContent = state.cardIndex === words.length - 1 ? "去阅读" : "下一个";
  renderMissionWords();
  renderQueue();
}

function markWordLearning(word) {
  if (word.status === "unlearned") {
    word.status = "learning";
  }
}

function listenCurrentWord() {
  const word = words[state.cardIndex];
  state.listened.add(word.id);
  if ("speechSynthesis" in window) {
    window.speechSynthesis.cancel();
    const utterance = new SpeechSynthesisUtterance(word.text);
    utterance.lang = "en-US";
    utterance.rate = 0.78;
    window.speechSynthesis.speak(utterance);
  }
  showToast(`正在播放：${word.text}`);
  renderCard();
}

function markReadDone() {
  const word = words[state.cardIndex];
  state.readDone.add(word.id);
  if (state.cardIndex < words.length - 1) {
    state.cardIndex += 1;
    renderCard();
    return;
  }
  unlockView("reading");
  switchView("reading");
}

function renderStory() {
  const story = `
    In the morning, I put an <mark>apple</mark> on my <mark>desk</mark>.
    I <mark>read</mark> a short note from my teacher.
    The note says, "Drink <mark>water</mark>, pack <mark>their</mark> books, and do not <mark>climb</mark> on chairs."
    I drink <mark>water</mark> and put another <mark>apple</mark> in my bag.
    My friends put <mark>their</mark> books on the <mark>desk</mark>.
    After class, we <mark>read</mark> the note again and <mark>climb</mark> the small hill near school.
  `;
  $("#storyText").innerHTML = story;
  $("#appearanceList").innerHTML = words.map((word) => {
    const count = (story.match(new RegExp(`<mark>${word.text}</mark>`, "g")) || []).length;
    return `<span>${word.text} 出现 ${count} 次</span>`;
  }).join("");
}

function renderDictation() {
  $("#dictationList").innerHTML = words.map((word) => {
    const value = state.answers[word.id] || "";
    const answer = value.trim().toLowerCase();
    const expected = word.text.toLowerCase();
    const resultClass = state.submitted ? (answer === expected ? "correct" : "wrong") : "";
    const isCorrectionTarget = state.correctionMode && state.correctionTargets.has(word.id);
    const hasBonus = state.submitted && state.firstAttemptBonus.has(word.id) && !state.correctionMode;
    const resultContent = hasBonus
      ? `<img class="bonus-reward" src="./assets/bonus-star.svg" alt="首次默写拼对奖励" />`
      : (!state.submitted ? "" : answer === expected ? "正确" : "需订正");
    const readonly = state.submitted || (state.correctionMode && !isCorrectionTarget);
    return `
      <div class="dictation-row ${resultClass} ${isCorrectionTarget ? "correction" : ""}" data-id="${word.id}">
        <div class="prompt">
          <strong>${word.meaningZh}</strong>
          <span>${state.showPhonetic ? word.phonetic : "音标已隐藏"}</span>
        </div>
        <div class="answer-box">
          <div class="${isCorrectionTarget ? "correction-input-shell" : ""}">
            ${isCorrectionTarget ? `<span class="ghost-spelling" aria-hidden="true">${escapeHtml(word.text)}</span>` : ""}
            <input value="${escapeAttr(value)}" ${readonly ? "readonly" : ""} placeholder="${isCorrectionTarget ? "" : "写出英文单词"}" data-answer="${word.id}" />
          </div>
        </div>
        <span class="result-label">${resultContent}</span>
      </div>
    `;
  }).join("");

  $$("[data-answer]").forEach((input) => {
    input.addEventListener("input", () => {
      state.answers[input.dataset.answer] = input.value;
    });
  });
}

function submitDictation() {
  $$("[data-answer]").forEach((input) => {
    state.answers[input.dataset.answer] = input.value;
  });
  if (state.correctionMode) {
    const wrong = getCorrectionWrongWords();
    if (wrong.length > 0) {
      state.submitted = false;
      renderDictation();
      showToast(`${wrong.length} 个词还没订正对，再照着影子字写一遍`);
      return;
    }
    state.submitted = true;
    renderDictation();
    showToast("订正完成");
    unlockView("done");
    window.setTimeout(() => switchView("done"), 700);
    return;
  }
  if (!state.firstDictationSubmitted && !state.correctionMode) {
    state.firstAttemptBonus = new Set(words
      .filter((word) =>
        word.type === "new" &&
        word.correctStreak === 0 &&
        word.lane === "first" &&
        (state.answers[word.id] || "").trim().toLowerCase() === word.text.toLowerCase()
      )
      .map((word) => word.id));
    applyDictationStatus();
    state.firstAttemptWrong = new Set(getWrongWords().map((word) => word.id));
    state.firstDictationSubmitted = true;
  }
  state.submitted = true;
  renderDictation();
  renderMissionWords();
  const wrong = state.correctionMode ? getCorrectionWrongWords() : getWrongWords();
  $("#retryWrongBtn").hidden = wrong.length === 0 || state.correctionMode;
  showToast(wrong.length === 0 ? "全部写对了" : `${wrong.length} 个词需要订正，可以回到默写页处理`);
  unlockView("done");
  window.setTimeout(() => switchView("done"), 700);
}

function applyDictationStatus() {
  words.forEach((word) => {
    const answer = (state.answers[word.id] || "").trim().toLowerCase();
    const correct = answer === word.text.toLowerCase();
    if (!correct) {
      word.status = "reinforce";
      word.correctStreak = 0;
      return;
    }
    word.correctStreak += 1;
    word.status = word.correctStreak >= 3 ? "mastered" : "learning";
  });
}

function retryWrong() {
  const wrongWords = getWrongWords();
  state.correctionMode = true;
  state.correctionTargets = new Set(wrongWords.map((word) => word.id));
  wrongWords.forEach((word) => {
    state.answers[word.id] = "";
  });
  state.submitted = false;
  $("#retryWrongBtn").hidden = true;
  renderDictation();
  showToast("看绿色提示，把错词再写一遍");
}

function getWrongWords() {
  return words.filter((word) => (state.answers[word.id] || "").trim().toLowerCase() !== word.text.toLowerCase());
}

function getCorrectionWrongWords() {
  return words.filter((word) =>
    state.correctionTargets.has(word.id) &&
    (state.answers[word.id] || "").trim().toLowerCase() !== word.text.toLowerCase()
  );
}

function renderDone() {
  const firstAttemptWrongCount = state.firstAttemptWrong.size;
  const firstAttemptWrongWords = words.filter((word) => state.firstAttemptWrong.has(word.id));
  const correct = words.length - firstAttemptWrongCount;
  $("#correctCount").textContent = String(correct);
  $("#wrongCount").textContent = String(firstAttemptWrongCount);
  $("#tomorrowCount").textContent = String(firstAttemptWrongCount || words.length);
  $("#summaryText").textContent = `完成 ${words.length} 个单词学习，首次默写正确 ${correct} 个。`;
  const tomorrow = firstAttemptWrongWords.length ? firstAttemptWrongWords : words;
  $("#tomorrowWords").innerHTML = tomorrow.map((word) => {
    const label = state.firstAttemptWrong.has(word.id) ? "首次拼错" : statusText[word.status];
    return `<span>${word.text} · ${label}</span>`;
  }).join("");
  renderCheckins();
}

function seedCheckins() {
  const today = new Date();
  for (let offset = 6; offset >= 1; offset -= 1) {
    const day = new Date(today);
    day.setDate(today.getDate() - offset);
    state.checkins.add(dateKey(day));
  }
}

function renderCheckins() {
  const today = new Date();
  const year = today.getFullYear();
  const month = today.getMonth();
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  $("#monthLabel").textContent = `${month + 1} 月`;
  $("#checkinSummary").textContent = `连续打卡 ${currentStreak()} 天`;
  $("#checkinGrid").innerHTML = Array.from({ length: daysInMonth }, (_, index) => {
    const date = new Date(year, month, index + 1);
    const key = dateKey(date);
    const checked = state.checkins.has(key);
    const isToday = key === dateKey(today);
    return `
      <span class="checkin-day ${checked ? "checked" : ""} ${isToday ? "today" : ""}">
        ${index + 1}
      </span>
    `;
  }).join("");
}

function completeToday() {
  state.checkins.add(dateKey(new Date()));
  renderCheckins();
  const streak = currentStreak();
  if (streak >= 7 && !state.checkinBonusShown) {
    state.checkinBonusShown = true;
    $("#checkinBonusModal").hidden = false;
    return;
  }
  showToast("今日已打卡");
}

function currentStreak() {
  let streak = 0;
  const cursor = new Date();
  for (;;) {
    if (!state.checkins.has(dateKey(cursor))) break;
    streak += 1;
    cursor.setDate(cursor.getDate() - 1);
  }
  return streak;
}

function dateKey(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function escapeAttr(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function bindEvents() {
  $("#todayText").textContent = new Date().toLocaleDateString("zh-CN", {
    month: "long",
    day: "numeric",
    weekday: "long"
  });
  seedCheckins();

  $$(".step").forEach((step) => {
    step.addEventListener("click", () => switchView(step.dataset.view));
  });
  $("#startBtn").addEventListener("click", () => {
    unlockView("cards");
    switchView("cards");
  });
  $("#listenBtn").addEventListener("click", listenCurrentWord);
  $("#readDoneBtn").addEventListener("click", markReadDone);
  $("#hardBtn").addEventListener("click", () => {
    const word = words[state.cardIndex];
    state.hardWords.add(word.id);
    word.status = "reinforce";
    word.correctStreak = 0;
    showToast("已标记为需要强化");
    renderCard();
  });
  $("#prevCardBtn").addEventListener("click", () => {
    state.cardIndex = Math.max(0, state.cardIndex - 1);
    renderCard();
  });
  $("#nextCardBtn").addEventListener("click", () => {
    if (state.cardIndex === words.length - 1) {
      unlockView("reading");
      switchView("reading");
      return;
    }
    state.cardIndex += 1;
    renderCard();
  });
  $("#listenStoryBtn").addEventListener("click", () => {
    if (!("speechSynthesis" in window)) {
      showToast("当前浏览器不支持语音播放");
      return;
    }
    window.speechSynthesis.cancel();
    const utterance = new SpeechSynthesisUtterance(words.map((word) => word.text).join(". "));
    utterance.lang = "en-US";
    utterance.rate = 0.75;
    window.speechSynthesis.speak(utterance);
    showToast("正在播放重点词");
  });
  $("#readingDoneBtn").addEventListener("click", () => {
    unlockView("dictation");
    switchView("dictation");
  });
  $("#togglePhoneticBtn").addEventListener("click", () => {
    state.showPhonetic = !state.showPhonetic;
    $("#togglePhoneticBtn").textContent = state.showPhonetic ? "隐藏音标提示" : "显示音标提示";
    renderDictation();
  });
  $("#submitDictationBtn").addEventListener("click", submitDictation);
  $("#retryWrongBtn").addEventListener("click", retryWrong);
  $("#finishBtn").addEventListener("click", completeToday);
  $("#closeCheckinBonusBtn").addEventListener("click", () => {
    $("#checkinBonusModal").hidden = true;
    showToast("奖励已收下");
  });
  renderSteps();
}

renderMissionWords();
bindEvents();
