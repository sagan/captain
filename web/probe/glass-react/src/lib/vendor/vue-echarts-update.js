// @ts-nocheck
/* Update planner from vue-echarts 8.3.2 (dist/update-*.js, region src/update.ts),
 * copied verbatim so the React chart issues exactly the setOption calls the
 * Vue build's <VChart> issues. Only isPlainObject is inlined from src/utils.ts.
 *
 * The MIT License (MIT)
 *
 * Copyright (c) 2016-present GU Yiling & ECOMFE
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
import { ComponentModel } from 'echarts/core'

function isPlainObject(value) {
	if (value == null || typeof value !== "object") return false;
	const prototype = Object.getPrototypeOf(value);
	return prototype === null || prototype === Object.prototype;
}
//#region src/update.ts
function toIdentity(value) {
	return typeof value === "string" || typeof value === "number" ? String(value) : void 0;
}
function buildShape(value, context, mode) {
	if (!isPlainObject(value)) return true;
	const shape = Object.create(null);
	for (const key of Object.keys(value)) {
		const child = value[key];
		if (child === void 0) continue;
		if (mode === "graphic" && key === "$action") context.hasAction = true;
		if (mode === "graphic" && (key === "elements" || key === "children") && Array.isArray(child)) {
			shape[key] = analyzeItems(child, context, "graphic", true);
			continue;
		}
		if (mode === "option") {
			const componentItems = ComponentModel.hasClass(key);
			if (Array.isArray(child) || componentItems && isPlainObject(child)) {
				shape[key] = analyzeItems(Array.isArray(child) ? child : [child], context, key === "graphic" ? "graphic" : void 0, componentItems);
				continue;
			}
		}
		shape[key] = mode === "media" && key === "option" ? buildShape(child, context, "option") : buildShape(child, context);
	}
	return shape;
}
function analyzeItems(items, context, mode, componentItems) {
	const shapes = [];
	for (const item of items) {
		const identity = componentItems ? item : void 0;
		shapes.push({
			id: toIdentity(identity?.id),
			name: toIdentity(identity?.name),
			...mode === "graphic" ? {
				type: toIdentity(identity?.type),
				parentId: toIdentity(identity?.parentId)
			} : {},
			shape: buildShape(item, context, mode)
		});
	}
	return shapes;
}
/**
* Build a structural signature that retains component identities but not option payload values.
* Data arrays inside component items remain leaves. Graphic element trees and nested option units
* (`baseOption`, timeline options, and media options) reuse collection summaries.
*/
function buildSignature(option) {
	const opt = option;
	const context = { hasAction: false };
	const collections = Object.create(null);
	const objectShapes = Object.create(null);
	const leaves = [];
	for (const key of Object.keys(opt)) {
		const value = opt[key];
		const componentItems = ComponentModel.hasClass(key);
		if (Array.isArray(value) || componentItems && isPlainObject(value)) {
			const mode = key === "options" ? "option" : key === "media" ? "media" : key === "graphic" ? "graphic" : void 0;
			collections[key] = analyzeItems(Array.isArray(value) ? value : [value], context, mode, componentItems);
			continue;
		}
		if (isPlainObject(value)) {
			objectShapes[key] = buildShape(value, context, key === "baseOption" ? "option" : void 0);
			continue;
		}
		if (value !== void 0 && !componentItems) leaves.push(key);
	}
	return {
		collections,
		objectShapes,
		leaves,
		hasAction: context.hasAction
	};
}
function hasCollectionRemoval(prev, next) {
	if (!next) return prev.length > 0;
	return next.length < prev.length;
}
function getItemIdentity(item) {
	return item.id !== void 0 ? `id:${item.id}` : item.name !== void 0 ? `name:${item.name}` : void 0;
}
function hasIdentityChange(prev, next) {
	const length = Math.min(prev.length, next.length);
	for (let index = 0; index < length; index++) if (getItemIdentity(prev[index]) !== getItemIdentity(next[index])) return true;
	return false;
}
function preservesReplacementOrder(prev, next) {
	const positions = /* @__PURE__ */ new Map();
	for (let index = 0; index < prev.length; index++) {
		const item = prev[index];
		if (item.id !== void 0) positions.set(item.id, index);
	}
	return next.every((item, index) => {
		const previousIndex = item.id === void 0 ? void 0 : positions.get(item.id);
		return previousIndex === void 0 || previousIndex === index;
	});
}
function hasShapeRemoval(prev, next) {
	if (prev === true || next === true) return false;
	const prevIsCollection = Array.isArray(prev);
	const nextIsCollection = Array.isArray(next);
	if (prevIsCollection || nextIsCollection) return !prevIsCollection || !nextIsCollection || hasCollectionRemoval(prev, next) || hasIdentityChange(prev, next) || compareItemShapes(prev, next) !== 0;
	for (const key in prev) {
		const nextChild = next[key];
		if (nextChild === void 0 || hasShapeRemoval(prev[key], nextChild)) return true;
	}
	return false;
}
function compareItemShapes(prev, next) {
	const byId = /* @__PURE__ */ new Map();
	for (const item of next) if (item.id !== void 0 && !byId.has(item.id)) byId.set(item.id, item);
	let kind = 0;
	for (let index = 0; index < prev.length; index++) {
		const item = prev[index];
		const candidate = item.id !== void 0 ? byId.get(item.id) : next[index];
		if (candidate && (item.type !== candidate.type || item.parentId !== candidate.parentId || hasShapeRemoval(item.shape, candidate.shape))) {
			if (item.id !== void 0) return 2;
			kind = 1;
		}
	}
	return kind;
}
function needsGlobalReset(prev, next) {
	if (next.objectShapes.aria && !prev.objectShapes.aria) return true;
	for (const key in next.collections) if (!prev.collections[key] && !prev.objectShapes[key] && !prev.leaves.includes(key) && !ComponentModel.hasClass(key)) return true;
	for (const key in prev.objectShapes) {
		const prevShape = prev.objectShapes[key];
		const nextShape = next.objectShapes[key];
		if (prevShape && nextShape) {
			if (hasShapeRemoval(prevShape, nextShape)) return true;
			continue;
		}
		if (!next.leaves.includes(key)) return true;
	}
	for (const key of prev.leaves) if (!next.leaves.includes(key) && next.collections[key] === void 0 && next.objectShapes[key] === void 0) return true;
	return false;
}
function compareCollection(prev, next) {
	const kind = next ? compareItemShapes(prev, next) : 0;
	if (kind === 2) return kind;
	if (kind === 0 && !hasCollectionRemoval(prev, next) && !(next && hasIdentityChange(prev, next))) return 0;
	return next && !preservesReplacementOrder(prev, next) ? 2 : 1;
}
/**
* Produce an update plan that preserves option structure and initialization semantics.
* Falls back to `notMerge: true` when the change looks complex.
*/
function planUpdate(prev, option) {
	const next = buildSignature(option);
	if (!prev) return {
		signature: next,
		plan: { notMerge: false }
	};
	let reset = needsGlobalReset(prev, next);
	const replaceMerge = [];
	for (const key in prev.collections) {
		const collection = next.collections[key];
		const kind = compareCollection(prev.collections[key], collection);
		if (!ComponentModel.hasClass(key)) reset ||= !collection || kind !== 0;
		else if (kind === 2) reset = true;
		else if (kind === 1) replaceMerge.push(key);
	}
	const replacements = next.hasAction ? replaceMerge.filter((key) => key !== "graphic") : replaceMerge;
	const notMerge = reset && !next.hasAction;
	return {
		signature: next,
		plan: !notMerge && replacements.length ? {
			notMerge,
			replaceMerge: replacements.sort()
		} : { notMerge }
	};
}
//#endregion
export { planUpdate }
