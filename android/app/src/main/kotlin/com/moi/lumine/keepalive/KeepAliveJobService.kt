package com.moi.lumine.keepalive

import android.app.job.JobInfo
import android.app.job.JobParameters
import android.app.job.JobScheduler
import android.app.job.JobService
import android.content.ComponentName
import android.content.Context
import android.util.Log

class KeepAliveJobService : JobService() {

    override fun onStartJob(params: JobParameters?): Boolean {
        Log.d("KeepAliveJob", "JobScheduler 触发，检查服务状态")

        if (!KeepAlive.shouldRun(this)) {
            jobFinished(params, false)
            return true
        }

        KeepAlive.tryRestart(this)
        jobFinished(params, false)
        return true
    }

    override fun onStopJob(params: JobParameters?): Boolean {
        return false
    }

    companion object {
        private const val JOB_ID = 10086

        fun schedule(context: Context) {
            val scheduler = context.getSystemService(Context.JOB_SCHEDULER_SERVICE) as JobScheduler
            if (scheduler.getPendingJob(JOB_ID) != null) return

            if (!KeepAlive.shouldRun(context)) return

            val jobInfo = JobInfo.Builder(JOB_ID, ComponentName(context, KeepAliveJobService::class.java))
                .setPeriodic(15 * 60 * 1000L)
                .setPersisted(true)
                .setRequiredNetworkType(JobInfo.NETWORK_TYPE_ANY)
                .build()

            val result = scheduler.schedule(jobInfo)
            Log.d("KeepAliveJob", "JobScheduler 调度: ${if (result == JobScheduler.RESULT_SUCCESS) "成功" else "失败"}")
        }

        fun cancel(context: Context) {
            val scheduler = context.getSystemService(Context.JOB_SCHEDULER_SERVICE) as JobScheduler
            scheduler.cancel(JOB_ID)
            Log.d("KeepAliveJob", "JobScheduler 已取消")
        }
    }
}
