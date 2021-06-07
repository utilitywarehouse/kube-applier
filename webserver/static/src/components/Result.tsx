import { Disclosure, Transition } from '@headlessui/react'
import classNames from 'classnames'
import { DateTime } from 'luxon'
import { ObjectMeta, WaybillSpec, WaybillStatus } from '../lib/spec'
interface Props {
    diffURL?: string;
    spec?: WaybillSpec;
    status?: WaybillStatus;
    metadata?: ObjectMeta;
    expanded?: boolean;
}

const Result: React.FC<Props> = ({ diffURL, spec, status, metadata, expanded }) => {
    const containerCx = classNames('flex flex-col rounded-sm border', {
        'border-green-500': status?.lastRun?.success,
        'border-red-600': !status?.lastRun?.success,
    })
    const headlineCx = classNames('flex items-center space-x-1 cursor-pointer text-white font-bold flex-1 p-3', {
        'bg-green-500': status?.lastRun?.success,
        'bg-red-600': !status?.lastRun?.success,
    })
    return (
        <Disclosure defaultOpen={expanded}>
            {({ open }) => (
                <div className={containerCx}>
                    <Disclosure.Button as="div" className={headlineCx}>
                        <div className="flex-1">{metadata?.namespace}</div>
                        {spec?.dryRun && <div className="uppercase border px-2 py-1 bg-red-600 border-red-600 rounded-sm text-xs">Dry run</div>}
                        {spec?.prune && <div className="uppercase border px-2 py-1 rounded-sm text-xs">Prune</div>}
                        {spec?.autoApply && <div className="uppercase border px-2 py-1 rounded-sm text-xs">Auto apply</div>}
                        {spec?.runInterval && <div className="border px-2 py-1 rounded-sm text-xs">INTERVAL {spec.runInterval}s</div>}
                    </Disclosure.Button>
                    <Transition
                        show={open}
                        enter="transition duration-100 ease-in"
                        enterFrom=" opacity-0"
                        enterTo=" opacity-100"
                        leave="transition duration-75 ease-in"
                        leaveFrom="opacity-100"
                        leaveTo="opacity-0">
                    <Disclosure.Panel static>
                        <div className="p-2">
                            {status?.lastRun && (
                                <>
                                    <div className="space-y-1">
                                        <div className="flex space-x-2 items-center">
                                            <p className="bg-gray-100 px-4 py-1 text-xs rounded-sm">{status?.lastRun?.type}</p>
                                            <p className="bg-gray-100 px-4 py-1 text-xs rounded-sm">Commit <a className="hover:underline text-blue-600 font-bold" href={diffURL}>({status?.lastRun?.commit})</a></p>
                                            <p className="bg-gray-100 px-4 py-1 text-xs rounded-sm">{DateTime.fromISO(status?.lastRun?.started).toRelative()} (took 10s)</p>
                                            <p className="flex-1" />
                                            <button className="bg-orange-400 border-orange-500 px-3 py-1 rounded-sm border hover:bg-orange-500 hover:border-orange-600 text-white font-bold text-sm">Force run</button>
                                        </div>
                                    </div>
                                    {!status?.lastRun?.success && (
                                        <div className="space-y-1 my-2">
                                            <p className="font-bold uppercase text-xs">Error</p>
                                            <pre className="bg-red-500 py-3 px-2 text-white overflow-hidden text-xs rounded-sm">{status?.lastRun?.errorMessage}</pre>
                                        </div>
                                    )}
                                    <details open={expanded} className="bg-gray-100 p-2 mt-2 overflow-auto text-xs border border-gray-200 rounded-sm">
                                        <summary>Output</summary>
                                        <p className="truncate py-2"><strong>$</strong> {status?.lastRun?.command}</p>
                                        <pre className="py-2">
                                            {status?.lastRun?.output}
                                        </pre>
                                    </details>
                                </>
                            )}
                        </div>
                    </Disclosure.Panel>
                    </Transition>
                </div>
            )}
        </Disclosure>
    )
}

export default Result
