#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const [openapiPath, outputPath] = process.argv.slice(2);
if (!openapiPath || !outputPath) {
	throw new Error("usage: generate-kotlin-contract.mjs <openapi.json> <ApiModels.kt>");
}

const document = JSON.parse(readFileSync(openapiPath, "utf8"));
const schemas = document.components?.schemas ?? {};
const actionNames = [
	"CardSyncAction",
	"CardOrderSyncAction",
	"ListSyncAction",
	"TeamSyncAction",
	"MemberSyncAction",
	"PreferenceSyncAction",
	"SyncStatusSyncAction",
	"MilestoneSyncAction"
];

const refName = (ref) => ref.split("/").at(-1);
const nullable = (schema) => schema?.nullable === true || schema?.type === "null" || schema?.anyOf?.some((entry) => entry.type === "null");

function kotlinType(schema, optional = false) {
	if (!schema) return "JsonElement" + (optional ? "?" : "");
	if (schema.$ref) {
		const name = refName(schema.$ref) === "AnySyncAction" ? "SyncAction" : refName(schema.$ref);
		return name + (optional || nullable(schema) ? "?" : "");
	}
	if (schema.anyOf || schema.oneOf) {
		const variants = (schema.anyOf ?? schema.oneOf).filter((entry) => entry.type !== "null");
		if (variants.length > 0 && variants.every((entry) => entry.$ref && actionNames.includes(refName(entry.$ref)))) {
			return "SyncAction" + (optional || nullable(schema) ? "?" : "");
		}
		if (variants.length === 1) return kotlinType(variants[0], optional || nullable(schema));
		return "JsonElement" + (optional || nullable(schema) ? "?" : "");
	}
	let result;
	switch (schema.type) {
		case "string":
			result = "String";
			break;
		case "integer":
			result = schema.format === "int64" ? "Long" : "Int";
			break;
		case "number":
			result = "Double";
			break;
		case "boolean":
			result = "Boolean";
			break;
		case "array":
			result = `List<${kotlinType(schema.items)}>`;
			break;
		case "object":
			result = "JsonObject";
			break;
		default:
			result = "JsonElement";
	}
	return result + (optional || nullable(schema) ? "?" : "");
}

function flattenObject(schema, seen = new Set()) {
	const properties = {};
	const required = new Set(schema.required ?? []);
	for (const part of schema.allOf ?? []) {
		if (part.$ref) {
			const name = refName(part.$ref);
			if (!seen.has(name)) {
				const next = new Set(seen).add(name);
				const flattened = flattenObject(schemas[name] ?? {}, next);
				Object.assign(properties, flattened.properties);
				for (const item of flattened.required) required.add(item);
			}
		} else {
			const flattened = flattenObject(part, seen);
			Object.assign(properties, flattened.properties);
			for (const item of flattened.required) required.add(item);
		}
	}
	Object.assign(properties, schema.properties ?? {});
	return { properties, required };
}

const identifier = (name) =>
	/^(as|break|class|continue|do|else|false|for|fun|if|in|interface|is|null|object|package|return|super|this|throw|true|try|typealias|typeof|val|var|when|while)$/.test(
		name
	)
		? `\`${name}\``
		: name;
const enumEntry = (value) => {
	const candidate = String(value)
		.replace(/[^A-Za-z0-9]+/g, "_")
		.replace(/^_+|_+$/g, "")
		.toUpperCase();
	return /^[0-9]/.test(candidate) ? `VALUE_${candidate}` : candidate || "UNKNOWN";
};

function renderModel(name, schema) {
	if (schema.enum?.length > 1 && schema.type === "string") {
		return `@Serializable\nenum class ${name} {\n${schema.enum.map((value) => `    @SerialName(${JSON.stringify(value)}) ${enumEntry(value)}`).join(",\n")}\n}`;
	}
	const { properties, required } = flattenObject(schema);
	if (Object.keys(properties).length === 0) {
		return `typealias ${name} = ${kotlinType(schema)}`;
	}
	const fields = Object.entries(properties).map(([property, propertySchema]) => {
		const isRequired = required.has(property);
		const annotation = identifier(property) === property ? "" : `    @SerialName(${JSON.stringify(property)})\n`;
		const defaultValue = isRequired ? "" : " = null";
		return `${annotation}    val ${identifier(property)}: ${kotlinType(propertySchema, !isRequired)}${defaultValue}`;
	});
	return `@Serializable\ndata class ${name}(\n${fields.join(",\n")}\n)`;
}

function renderSyncActions() {
	const definitions = actionNames.map((name) => {
		const { properties, required } = flattenObject(schemas[name] ?? {});
		const entity = properties.entity?.enum?.[0] ?? name;
		delete properties.entity;
		const fields = Object.entries(properties).map(([property, propertySchema]) => {
			const common = ["syncId", "entityId", "operation", "actorGitLabUserId", "occurredAt"].includes(property);
			const isRequired = required.has(property);
			return `    ${common ? "override " : ""}val ${identifier(property)}: ${kotlinType(propertySchema, !isRequired)}${isRequired ? "" : " = null"}`;
		});
		return `@Serializable\n@SerialName(${JSON.stringify(entity)})\ndata class ${name}(\n${fields.join(",\n")}\n) : SyncAction`;
	});
	return `@Serializable\n@JsonClassDiscriminator("entity")\nsealed interface SyncAction {\n    val syncId: String\n    val entityId: String\n    val operation: String\n    val actorGitLabUserId: Long?\n    val occurredAt: String\n}\n\n${definitions.join("\n\n")}`;
}

const skip = new Set(["SyncAction", "AnySyncAction", ...actionNames]);
const rendered = Object.entries(schemas)
	.filter(([name]) => !skip.has(name))
	.map(([name, schema]) => renderModel(name, schema));

const source = `// Generated from api/**/*.tsp via OpenAPI. DO NOT EDIT.\n@file:OptIn(ExperimentalSerializationApi::class)\n\npackage org.sitcon.sitlab.api.generated\n\nimport kotlinx.serialization.ExperimentalSerializationApi\nimport kotlinx.serialization.SerialName\nimport kotlinx.serialization.Serializable\nimport kotlinx.serialization.json.JsonClassDiscriminator\nimport kotlinx.serialization.json.JsonElement\nimport kotlinx.serialization.json.JsonObject\n\n${renderSyncActions()}\n\n${rendered.join("\n\n")}\n`;

mkdirSync(path.dirname(outputPath), { recursive: true });
writeFileSync(outputPath, source);
