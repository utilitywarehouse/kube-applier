import { ObjectMeta, WaybillSpec, WaybillStatus } from '../lib/spec'
import { Disclosure, Transition } from '@headlessui/react'
import classNames from 'classnames'

interface Props {
    diffURL: string;
    spec: WaybillSpec;
    status: WaybillStatus;
    metadata: ObjectMeta;
}

const Result: React.FC<Props> = ({ diffURL, spec, status, metadata }) => {
    const containerCx = classNames('flex flex-col rounded border', {
        'border-green-600': status?.lastRun?.success,
        'border-red-600': !status?.lastRun?.success,
    })
    const headlineCx = classNames('flex items-center space-x-1 cursor-pointer text-white font-bold flex-1 p-2', {
        'bg-green-600': status?.lastRun?.success,
        'bg-red-600': !status?.lastRun?.success,
    })
    return (
        <Disclosure>
            {({ open }) => (
                <div className={containerCx}>
                    <Disclosure.Button as="div" className={headlineCx}>
                        <div className="flex-1">{metadata?.namespace}</div>
                        {spec?.dryRun && <div className="uppercase border p-1 bg-red-500 border-red-800 rounded text-xs">Dry run</div>}
                        {spec?.prune && <div className="uppercase border p-1 rounded text-xs">Prune</div>}
                        {spec?.autoApply && <div className="uppercase border p-1 rounded text-xs">Auto apply</div>}
                    </Disclosure.Button>
                    <Transition
                        show={open}
                        enter="transition duration-100 ease-in"
                        enterFrom=" opacity-0"
                        enterTo=" opacity-100"
                        leave="transition duration-75 ease-in"
                        leaveFrom="opacity-100"
                        leaveTo="opacity-0">
                    <Disclosure.Panel static >
                        <div className="p-4">
                            <div className="space-y-1">
                                <p><strong>Type</strong>: {status?.lastRun?.type}</p>
                                <p><strong>Started</strong>: {status?.lastRun?.started}</p>
                                <p><strong>Finished</strong>: {status?.lastRun?.finished}</p>
                            </div>
                            <div className="">
                                <p className="font-bold uppercase">Command</p>
                                <p>{status?.lastRun?.command}</p>
                            </div>
                            <p className="mt-4">Last Commit <a className="hover:underline text-blue-600 font-bold" href={diffURL}>(see diff)</a></p>
                            <pre className="bg-gray-100 p-4 overflow-hidden text-xs border border-gray-300 rounded">
                                {status?.lastRun?.output}
                            </pre>
                        </div>
                    </Disclosure.Panel>
                    </Transition>
                </div>
            )}
        </Disclosure>
    )
}

export default Result
