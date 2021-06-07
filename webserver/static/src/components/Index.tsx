import { useMemo, useState } from 'react';
import useFetch from 'use-http';
import Fuse from 'fuse.js';
import { Waybill } from '../lib/spec';
import Result from './Result'

const Index: React.FC = () => {
  const [searchValue, setSearchValue] = useState('')

  const { data } = useFetch<{
    Waybills: Waybill[];
    DiffURLFormat: string;
  }>('http://localhost:8080/api/v1/status', [])

	const availableValues = useMemo(() => {
    const fuse = new Fuse(data?.Waybills || [], {
      threshold: 0.2,
      keys: ['metadata.namespace']
    })
    return fuse.search(searchValue)
  }, [searchValue, data?.Waybills])

  return (
    <div className="flex flex-col">
      <input
        type="text"
        className="border flex-1 bg-gray-100 border-gray-200 p-3 mb-4 focus:outline-none"
        placeholder="Search"
        autoFocus
        onChange={(e) => setSearchValue(e.target.value)}
      />
      <div className="space-y-2">
        {availableValues?.map(({ item }, i) => (
          <Result
            key={item.metadata.namespace+i}
            diffURL={data?.DiffURLFormat}
            status={item.status}
            spec={item.spec}
            metadata={item.metadata}
          />
        ))}
      </div>
    </div>
  )
}

export default Index
