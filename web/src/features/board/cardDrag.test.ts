import { describe, expect, it } from "vitest";
import { allowsCardDragActivation } from "./cardDrag";

function buildCard() {
	const card = document.createElement("article");
	card.innerHTML = `
		<div class="actions">
			<button type="button" class="handle"><svg></svg></button>
			<a class="external" href="https://gitlab.example.com"><svg></svg></a>
		</div>
		<button type="button" class="title" data-card-drag-surface="true">
			<h3><span class="iid">#127</span>修正報名系統寄信流程</h3>
			<p class="description">釐清失敗重送條件</p>
		</button>
		<div class="labels" tabindex="0"><span class="chip">Backend</span></div>
		<footer>
			<label class="date"><input type="date" /></label>
			<button type="button" class="assignee">變更 Assignee</button>
		</footer>`;
	const handle = card.querySelector<HTMLElement>(".handle")!;
	return { card, handle, source: { element: card, handle } };
}

describe("allowsCardDragActivation", () => {
	const { card, handle, source } = buildCard();
	const at = (selector: string) => allowsCardDragActivation(card.querySelector(selector), source);

	it("activates on the card surface, the handle, and the labels strip", () => {
		expect(allowsCardDragActivation(card, source)).toBe(true);
		expect(allowsCardDragActivation(handle, source)).toBe(true);
		expect(at(".handle svg")).toBe(true);
		expect(at(".labels")).toBe(true);
		expect(at(".chip")).toBe(true);
	});

	it("activates on the title button because a short press still opens the card", () => {
		expect(at(".title")).toBe(true);
		expect(at(".title h3")).toBe(true);
		expect(at(".iid")).toBe(true);
		expect(at(".description")).toBe(true);
	});

	it("leaves interactive controls to their own gestures", () => {
		expect(at(".date input")).toBe(false);
		expect(at(".assignee")).toBe(false);
		expect(at(".external")).toBe(false);
		expect(at(".external svg")).toBe(false);
	});

	it("rejects non-element targets", () => {
		expect(allowsCardDragActivation(null, source)).toBe(false);
		expect(allowsCardDragActivation(document, source)).toBe(false);
	});
});
