import AbstractService from './abstractService'
import AttachmentModel from '../models/attachment'

import type { IAttachment } from '@/modelTypes/IAttachment'

import {downloadBlob} from '@/helpers/downloadBlob'
import {objectToCamelCase} from '@/helpers/case'

export enum PREVIEW_SIZE {
	SM = 'sm',
	MD = 'md',
	LG = 'lg',
	XL = 'xl',
}

export default class AttachmentService extends AbstractService<IAttachment> {
	constructor() {
		super({
			create: '/tasks/{taskId}/attachments',
			getAll: '/tasks/{taskId}/attachments',
			delete: '/tasks/{taskId}/attachments/{id}',
		})
	}

	processModel(model: IAttachment) {
		return {
			...model,
			created: new Date(model.created).toISOString(),
		}
	}

	useCreateInterceptor() {
		return false
	}

	modelFactory(data: Partial<IAttachment>) {
		return new AttachmentModel(data)
	}

	modelCreateFactory(data) {
		// Success contains the uploaded attachments
		data.success = (data.success === null ? [] : data.success).map(a => {
			return this.modelFactory(a)
		})
		return data
	}

	getBlobUrl(model: IAttachment, size?: PREVIEW_SIZE) {
		let mainUrl = '/tasks/' + model.taskId + '/attachments/' + model.id
		if (size !== undefined) {
			mainUrl += `?preview_size=${size}`
		}

		return AbstractService.prototype.getBlobUrl.call(this, mainUrl)
	}

	/**
	 * Modified by mia·nube on 2026-08-03.
	 *
	 * Attaches a file held by an external system as a reference rather than
	 * uploading a copy of it. The request carries the user's own Vikunja
	 * credentials like every other call from this service, so the server applies
	 * the same write permission an upload needs — the picker that produced the
	 * selection grants nothing on its own.
	 */
	async createLink(taskId: number, link: {
		provider: string,
		ref: string,
		name: string,
		size: number,
		mime: string,
	}): Promise<IAttachment> {
		// Spelled out rather than run through objectToSnakeCase: this service
		// disables the create interceptor (see useCreateInterceptor above), so a
		// camelCase body would reach the API unconverted and silently arrive as an
		// empty reference.
		const response = await this.http.put(`/tasks/${taskId}/attachments/link`, {
			link_provider: link.provider,
			link_ref: link.ref,
			link_name: link.name,
			link_size: link.size,
			link_mime: link.mime,
		})
		return this.modelFactory(objectToCamelCase(response.data))
	}

	async download(model: IAttachment) {
		const url = await this.getBlobUrl(model)
		return downloadBlob(url, model.file.name)
	}

	/**
	 * Uploads a file to the server
	 * @param files
	 * @returns {Promise<any|never>}
	 */
	create(model: IAttachment, files: File[] | FileList) {
		const data = new FormData()
		for (let i = 0; i < files.length; i++) {
			// TODO: Validation of file size
			data.append('files', new Blob([files[i]]), files[i].name)
		}

		return this.uploadFormData(
			this.getReplacedRoute(this.paths.create, model),
			data,
		)
	}
}
