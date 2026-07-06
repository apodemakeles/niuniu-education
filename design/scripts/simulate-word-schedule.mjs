#!/usr/bin/env node

const DAYS = 180;
const WORD_COUNT = 200;
const SEEDS = [11, 23, 37, 51, 79, 101, 137, 173];

const fixed = {
  newPoolMax: 3,
  reviewPoolMax: 9,
  masteryTarget: 3,
  dueTodayBonus: 40,
  overdueDailyBonus: 8,
  overdueBonusCap: 56,
};

const childModel = {
  unlearned: 0.82,
  learning: 0.9,
  reinforce: 0.78,
  mastered: 0.97,
  progressBonus: 0.035,
  overduePenalty: 0.018,
  longGapPenalty: 0.003,
  minSuccessRate: 0.5,
  maxSuccessRate: 0.98,
};

const parameterGrid = [];
for (const threshold of [70, 80, 90]) {
  for (const reinforceBase of [90, 100, 110]) {
    for (const learningBase of [60, 70, 80]) {
      for (const masteredBase of [0, 10, 20]) {
        for (const reinforceSuccessInterval of [1, 2]) {
          for (const learningIntervals of [[1, 2], [1, 3], [2, 4]]) {
            for (const masteredIntervals of [[60, 90, 150, 240], [90, 150, 240, 360], [120, 180, 300, 420]]) {
              for (const masteredDueSampleRatio of [0.01, 0.03, 0.05]) {
                for (const masteredSampleStopDay of [120, 140, 150, 160, 181]) {
                  parameterGrid.push({
                    threshold,
                    reinforceBase,
                    learningBase,
                    masteredBase,
                    reinforceSuccessInterval,
                    learningIntervals,
                    masteredIntervals,
                    masteredDueSampleRatio,
                    masteredSampleStopDay,
                  });
                }
              }
            }
          }
        }
      }
    }
  }
}

function mulberry32(seed) {
  let t = seed >>> 0;
  return function next() {
    t += 0x6d2b79f5;
    let x = t;
    x = Math.imul(x ^ (x >>> 15), x | 1);
    x ^= x + Math.imul(x ^ (x >>> 7), x | 61);
    return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
  };
}

function makeWords() {
  return Array.from({ length: WORD_COUNT }, (_, index) => ({
    id: index + 1,
    state: "unlearned",
    progress: 0,
    studiedDates: [],
    successDates: [],
    lastStudiedDay: null,
    lastSuccessDay: null,
    nextDueDay: null,
    masteredReviewSuccesses: 0,
    firstMasteredDay: null,
  }));
}

function shuffle(items, random) {
  const out = items.slice();
  for (let i = out.length - 1; i > 0; i -= 1) {
    const j = Math.floor(random() * (i + 1));
    [out[i], out[j]] = [out[j], out[i]];
  }
  return out;
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function successProbability(word, day) {
  const base = childModel[word.state] ?? childModel.learning;
  const progressBoost = word.state === "mastered" ? 0 : word.progress * childModel.progressBonus;
  const overdue = word.nextDueDay == null ? 0 : Math.max(0, day - word.nextDueDay);
  const lastStudyGap = word.lastStudiedDay == null ? 0 : Math.max(0, day - word.lastStudiedDay);
  const longGap = Math.max(0, lastStudyGap - 7);
  return clamp(
    base + progressBoost - overdue * childModel.overduePenalty - longGap * childModel.longGapPenalty,
    childModel.minSuccessRate,
    childModel.maxSuccessRate,
  );
}

function reviewScore(word, day, params) {
  const base = {
    reinforce: params.reinforceBase,
    learning: params.learningBase,
    mastered: params.masteredBase,
  }[word.state] ?? 0;
  const overdue = Math.max(0, day - (word.nextDueDay ?? day));
  return base + fixed.dueTodayBonus + Math.min(overdue * fixed.overdueDailyBonus, fixed.overdueBonusCap);
}

function isReviewCandidate(word, day, params, random) {
  if (word.state === "unlearned" || word.nextDueDay == null || word.nextDueDay > day) return false;
  if (word.state === "mastered") {
    if (day > params.masteredSampleStopDay) return false;
    return random() < params.masteredDueSampleRatio;
  }
  return true;
}

function pickNewWords(words, random) {
  const pool = words.filter((word) => word.state === "unlearned");
  return shuffle(pool, random).slice(0, fixed.newPoolMax);
}

function pickReviewWords(words, day, params, random) {
  return words
    .filter((word) => isReviewCandidate(word, day, params, random))
    .map((word) => ({ word, score: reviewScore(word, day, params) }))
    .filter((item) => item.score >= params.threshold)
    .sort((a, b) => b.score - a.score || a.word.nextDueDay - b.word.nextDueDay || a.word.id - b.word.id)
    .slice(0, fixed.reviewPoolMax)
    .map((item) => item.word);
}

function nextLearningInterval(progress, params) {
  if (progress <= 1) return params.learningIntervals[0];
  return params.learningIntervals[1];
}

function nextMasteredInterval(word, params) {
  const index = Math.min(word.masteredReviewSuccesses, params.masteredIntervals.length - 1);
  return params.masteredIntervals[index];
}

function applyStudyResult(word, day, success, params) {
  word.studiedDates.push(day);
  word.lastStudiedDay = day;

  if (!success) {
    word.state = "reinforce";
    word.progress = 0;
    word.masteredReviewSuccesses = 0;
    word.nextDueDay = day + 1;
    return;
  }

  word.successDates.push(day);
  word.lastSuccessDay = day;

  if (word.state === "mastered") {
    word.masteredReviewSuccesses += 1;
    word.nextDueDay = day + nextMasteredInterval(word, params);
    return;
  }

  word.progress += 1;
  if (word.progress >= fixed.masteryTarget) {
    word.state = "mastered";
    word.progress = fixed.masteryTarget;
    if (word.firstMasteredDay == null) word.firstMasteredDay = day;
    word.masteredReviewSuccesses = 0;
    word.nextDueDay = day + nextMasteredInterval(word, params);
    return;
  }

  if (word.state === "reinforce") {
    word.nextDueDay = day + params.reinforceSuccessInterval;
    return;
  }

  word.state = "learning";
  word.nextDueDay = day + nextLearningInterval(word.progress, params);
}

function simulateOnce(params, seed) {
  const random = mulberry32(seed);
  const words = makeWords();
  const daily = [];
  let allMasteredDay = null;
  let allEverMasteredDay = null;

  for (let day = 1; day <= DAYS; day += 1) {
    const newWords = pickNewWords(words, random);
    const reviewWords = pickReviewWords(words, day, params, random);
    const selected = [...newWords, ...reviewWords];

    let successes = 0;
    let failures = 0;

    for (const word of selected) {
      if (word.state === "unlearned") {
        word.state = "learning";
        word.nextDueDay = day;
      }
      const probability = successProbability(word, day);
      const success = random() < probability;
      if (success) successes += 1;
      else failures += 1;
      applyStudyResult(word, day, success, params);
    }

    const counts = countStates(words);
    const dueReviewCount = words.filter((word) =>
      word.state !== "unlearned" && word.nextDueDay != null && word.nextDueDay <= day + 1
    ).length;
    daily.push({
      day,
      newCount: newWords.length,
      reviewCount: reviewWords.length,
      total: selected.length,
      successes,
      failures,
      dueTomorrow: dueReviewCount,
      ...counts,
    });

    if (allMasteredDay == null && counts.mastered === WORD_COUNT) {
      allMasteredDay = day;
    }
    if (allEverMasteredDay == null && words.every((word) => word.firstMasteredDay != null)) {
      allEverMasteredDay = day;
    }
  }

  const finalCounts = countStates(words);
  const avgDaily = average(daily.map((d) => d.total));
  const avgReview = average(daily.map((d) => d.reviewCount));
  const maxDaily = Math.max(...daily.map((d) => d.total));
  const zeroReviewDays = daily.filter((d) => d.reviewCount === 0).length;
  const fullReviewDays = daily.filter((d) => d.reviewCount === fixed.reviewPoolMax).length;
  const totalFailures = daily.reduce((sum, d) => sum + d.failures, 0);

  return {
    allMasteredDay,
    allEverMasteredDay,
    finalCounts,
    avgDaily,
    avgReview,
    maxDaily,
    zeroReviewDays,
    fullReviewDays,
    totalFailures,
    daily,
  };
}

function countStates(words) {
  return words.reduce(
    (acc, word) => {
      acc[word.state] += 1;
      return acc;
    },
    { unlearned: 0, learning: 0, reinforce: 0, mastered: 0 },
  );
}

function average(values) {
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function summarizeParams(params) {
  const runs = SEEDS.map((seed) => simulateOnce(params, seed));
  const masteredDays = runs.map((run) => run.allMasteredDay ?? DAYS + 999);
  const everMasteredDays = runs.map((run) => run.allEverMasteredDay ?? DAYS + 999);
  const successRuns = runs.filter((run) => run.finalCounts.mastered === WORD_COUNT).length;
  const everSuccessRuns = runs.filter((run) => run.allEverMasteredDay != null && run.allEverMasteredDay <= DAYS).length;
  const avgMasteredDay = average(masteredDays);
  const avgEverMasteredDay = average(everMasteredDays);
  const worstMasteredDay = Math.max(...masteredDays);
  const worstEverMasteredDay = Math.max(...everMasteredDays);
  const avgFinalMastered = average(runs.map((run) => run.finalCounts.mastered));
  const avgDaily = average(runs.map((run) => run.avgDaily));
  const avgReview = average(runs.map((run) => run.avgReview));
  const avgZeroReviewDays = average(runs.map((run) => run.zeroReviewDays));
  const avgFullReviewDays = average(runs.map((run) => run.fullReviewDays));
  const avgFailures = average(runs.map((run) => run.totalFailures));
  const objective =
    (successRuns === SEEDS.length ? 0 : 100000)
    + (everSuccessRuns === SEEDS.length ? 0 : 20000)
    + worstMasteredDay * 10
    + (WORD_COUNT - avgFinalMastered) * 500
    + avgDaily * 5
    + avgFailures * 0.05
    + avgFullReviewDays * 0.2;
  return {
    params,
    successRuns,
    everSuccessRuns,
    avgMasteredDay,
    avgEverMasteredDay,
    worstMasteredDay,
    worstEverMasteredDay,
    avgFinalMastered,
    avgDaily,
    avgReview,
    avgZeroReviewDays,
    avgFullReviewDays,
    avgFailures,
    objective,
  };
}

function compactParams(params) {
  return {
    threshold: params.threshold,
    reinforceBase: params.reinforceBase,
    learningBase: params.learningBase,
    masteredBase: params.masteredBase,
    reinforceSuccessInterval: params.reinforceSuccessInterval,
    learningIntervals: params.learningIntervals.join("/"),
    masteredIntervals: params.masteredIntervals.join("/"),
    masteredDueSampleRatio: params.masteredDueSampleRatio,
    masteredSampleStopDay: params.masteredSampleStopDay,
  };
}

function printTop(results, count = 10) {
  console.log(`Simulated ${parameterGrid.length} parameter sets x ${SEEDS.length} seeds`);
  console.log(`Fixed: newPoolMax=${fixed.newPoolMax}, reviewPoolMax=${fixed.reviewPoolMax}, words=${WORD_COUNT}, days=${DAYS}`);
  console.log("Child model:", childModel);
  console.log("");
  for (const [index, result] of results.slice(0, count).entries()) {
    console.log(`#${index + 1}`, compactParams(result.params));
    console.log({
      successRuns: `${result.successRuns}/${SEEDS.length}`,
      everSuccessRuns: `${result.everSuccessRuns}/${SEEDS.length}`,
      avgMasteredDay: round(result.avgMasteredDay),
      avgEverMasteredDay: round(result.avgEverMasteredDay),
      worstMasteredDay: round(result.worstMasteredDay),
      worstEverMasteredDay: round(result.worstEverMasteredDay),
      avgFinalMastered: round(result.avgFinalMastered),
      avgDaily: round(result.avgDaily),
      avgReview: round(result.avgReview),
      avgZeroReviewDays: round(result.avgZeroReviewDays),
      avgFullReviewDays: round(result.avgFullReviewDays),
      avgFailures: round(result.avgFailures),
    });
  }
}

function printDailySample(result) {
  const run = simulateOnce(result.params, SEEDS[0]);
  const selectedDays = [1, 7, 14, 30, 60, 90, 120, 150, 180];
  console.log("");
  console.log("Daily sample for best params, seed", SEEDS[0]);
  for (const day of selectedDays) {
    const d = run.daily[day - 1];
    console.log(`day ${day}`, d);
  }
}

function printScenarioComparison(baseParams) {
  const scenarios = [
    { masteredDueSampleRatio: 0.01, masteredSampleStopDay: 120 },
    { masteredDueSampleRatio: 0.03, masteredSampleStopDay: 120 },
    { masteredDueSampleRatio: 0.05, masteredSampleStopDay: 120 },
    { masteredDueSampleRatio: 0.01, masteredSampleStopDay: 150 },
    { masteredDueSampleRatio: 0.03, masteredSampleStopDay: 150 },
    { masteredDueSampleRatio: 0.05, masteredSampleStopDay: 150 },
    { masteredDueSampleRatio: 0.01, masteredSampleStopDay: 181 },
    { masteredDueSampleRatio: 0.03, masteredSampleStopDay: 181 },
    { masteredDueSampleRatio: 0.05, masteredSampleStopDay: 181 },
  ];

  console.log("");
  console.log("Scenario comparison for selected base params");
  for (const scenario of scenarios) {
    const result = summarizeParams({ ...baseParams, ...scenario });
    console.log({
      masteredDueSampleRatio: scenario.masteredDueSampleRatio,
      masteredSampleStopDay: scenario.masteredSampleStopDay,
      successRuns: `${result.successRuns}/${SEEDS.length}`,
      avgFinalMastered: round(result.avgFinalMastered),
      worstMasteredDay: round(result.worstMasteredDay),
      avgDaily: round(result.avgDaily),
      avgReview: round(result.avgReview),
      avgFailures: round(result.avgFailures),
    });
  }
}

function round(value) {
  return Math.round(value * 100) / 100;
}

const results = parameterGrid
  .map(summarizeParams)
  .sort((a, b) => a.objective - b.objective);

printTop(results, 12);
printDailySample(results[0]);
printScenarioComparison(results[0].params);
